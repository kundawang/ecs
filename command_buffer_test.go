package ecs_test

import (
	"testing"

	"github.com/andygeiss/ecs"
)

// Each component type owns one bit. maskHealth and maskLoot belong to the
// two-system frame test; maskMana serves the single-buffer tests.
const (
	maskHealth = uint64(1 << 0)
	maskLoot   = uint64(1 << 1)
	maskMana   = uint64(1 << 2)
)

type health struct{ points int }

func (h *health) Mask() uint64 { return maskHealth }

type loot struct{}

func (l *loot) Mask() uint64 { return maskLoot }

type mana struct{ points int }

func (m *mana) Mask() uint64 { return maskMana }

func TestCommandBuffer_Queued_Commands_Should_Not_Touch_The_Manager_Until_Flush(t *testing.T) {
	t.Parallel()
	em := ecs.NewEntityManager()
	em.Add(ecs.NewEntity("enemy", []ecs.Component{&health{points: 0}}))
	buffer := ecs.NewCommandBuffer()
	buffer.CreateEntity(ecs.NewEntity("drop", []ecs.Component{&loot{}}))
	buffer.DestroyEntity("enemy")
	if em.Get("enemy") == nil {
		t.Error("enemy should still be there before Flush")
	}
	if em.Get("drop") != nil {
		t.Error("drop should not be there before Flush")
	}
	if err := buffer.Flush(em); err != nil {
		t.Fatalf("Flush should not fail, but got %v", err)
	}
	if em.Get("enemy") != nil {
		t.Error("enemy should be gone after Flush")
	}
	if em.Get("drop") == nil {
		t.Error("drop should be there after Flush")
	}
}

func TestCommandBuffer_Flush_Twice_Should_Apply_The_Batch_Once(t *testing.T) {
	t.Parallel()
	em := ecs.NewEntityManager()
	buffer := ecs.NewCommandBuffer()
	buffer.CreateEntity(ecs.NewEntity("drop", []ecs.Component{&loot{}}))
	if err := buffer.Flush(em); err != nil {
		t.Fatalf("Flush should not fail, but got %v", err)
	}
	if buffer.Len() != 0 {
		t.Errorf("buffer should be empty after Flush, but holds %d commands", buffer.Len())
	}
	if err := buffer.Flush(em); err != nil {
		t.Fatalf("second Flush should not fail, but got %v", err)
	}
	if got := len(em.FilterByMask(maskLoot)); got != 1 {
		t.Errorf("one drop should be there, but got %d", got)
	}
}

func TestCommandBuffer_Destroy_Unknown_Entity_Should_Be_A_NoOp(t *testing.T) {
	t.Parallel()
	em := ecs.NewEntityManager()
	buffer := ecs.NewCommandBuffer()
	buffer.DestroyEntity("ghost")
	if err := buffer.Flush(em); err != nil {
		t.Fatalf("Flush should not fail, but got %v", err)
	}
	if len(em.Entities()) != 0 {
		t.Errorf("manager should be empty, but holds %d entities", len(em.Entities()))
	}
}

func TestCommandBuffer_Destroy_Twice_Should_Apply_Once(t *testing.T) {
	t.Parallel()
	em := ecs.NewEntityManager()
	em.Add(ecs.NewEntity("enemy", []ecs.Component{&health{}}))
	buffer := ecs.NewCommandBuffer()
	buffer.DestroyEntity("enemy")
	buffer.DestroyEntity("enemy")
	if err := buffer.Flush(em); err != nil {
		t.Fatalf("Flush should not fail, but got %v", err)
	}
	if em.Get("enemy") != nil {
		t.Error("enemy should be gone after Flush")
	}
}

func TestCommandBuffer_Create_Then_Destroy_Same_Frame_Should_Cancel_Out(t *testing.T) {
	t.Parallel()
	em := ecs.NewEntityManager()
	buffer := ecs.NewCommandBuffer()
	buffer.CreateEntity(ecs.NewEntity("drop", []ecs.Component{&loot{}}))
	buffer.DestroyEntity("drop")
	if err := buffer.Flush(em); err != nil {
		t.Fatalf("Flush should not fail, but got %v", err)
	}
	if em.Get("drop") != nil {
		t.Error("a create cancelled by a destroy should leave nothing behind")
	}
}

func TestCommandBuffer_Destroy_Then_Create_Same_Id_Should_Recycle_The_Id(t *testing.T) {
	t.Parallel()
	em := ecs.NewEntityManager()
	em.Add(ecs.NewEntity("enemy", []ecs.Component{&health{points: 0}}))
	buffer := ecs.NewCommandBuffer()
	buffer.DestroyEntity("enemy")
	buffer.CreateEntity(ecs.NewEntity("enemy", []ecs.Component{&loot{}}))
	if err := buffer.Flush(em); err != nil {
		t.Fatalf("Flush should not fail, but got %v", err)
	}
	e := em.Get("enemy")
	if e == nil {
		t.Fatal("the recycled id should be there after Flush")
	}
	if e.Get(maskLoot) == nil {
		t.Error("the recycled entity should be the new one, carrying loot")
	}
	if e.Get(maskHealth) != nil {
		t.Error("the recycled entity should not carry the old health")
	}
}

func TestCommandBuffer_Create_Duplicate_Id_Should_Let_The_First_Claim_Win(t *testing.T) {
	t.Parallel()
	em := ecs.NewEntityManager()
	buffer := ecs.NewCommandBuffer()
	buffer.CreateEntity(ecs.NewEntity("drop", []ecs.Component{&loot{}}))
	buffer.CreateEntity(ecs.NewEntity("drop", []ecs.Component{&mana{points: 5}}))
	if err := buffer.Flush(em); err != nil {
		t.Fatalf("Flush should not fail, but got %v", err)
	}
	e := em.Get("drop")
	if e == nil {
		t.Fatal("drop should be there after Flush")
	}
	if e.Get(maskLoot) == nil {
		t.Error("the first create should win, carrying loot")
	}
	if e.Get(maskMana) != nil {
		t.Error("the second create should be dropped")
	}
	if got := len(em.Entities()); got != 1 {
		t.Errorf("one entity should be there, but got %d", got)
	}
}

func TestCommandBuffer_Component_Change_On_Destroyed_Entity_Should_Be_A_NoOp(t *testing.T) {
	t.Parallel()
	em := ecs.NewEntityManager()
	em.Add(ecs.NewEntity("enemy", []ecs.Component{&health{}}))
	buffer := ecs.NewCommandBuffer()
	buffer.DestroyEntity("enemy")
	buffer.AddComponent("enemy", &mana{points: 5})
	buffer.RemoveComponent("enemy", maskHealth)
	if err := buffer.Flush(em); err != nil {
		t.Fatalf("Flush should not fail, but got %v", err)
	}
	if em.Get("enemy") != nil {
		t.Error("enemy should be gone after Flush")
	}
}

func TestCommandBuffer_Component_Change_On_Created_Entity_Same_Frame_Should_Apply(t *testing.T) {
	t.Parallel()
	em := ecs.NewEntityManager()
	buffer := ecs.NewCommandBuffer()
	buffer.CreateEntity(ecs.NewEntity("drop", []ecs.Component{&loot{}}))
	buffer.AddComponent("drop", &mana{points: 5})
	if err := buffer.Flush(em); err != nil {
		t.Fatalf("Flush should not fail, but got %v", err)
	}
	e := em.Get("drop")
	if e == nil {
		t.Fatal("drop should be there after Flush")
	}
	if got := e.Get(maskMana).(*mana).points; got != 5 {
		t.Errorf("mana should be 5, but got %d", got)
	}
}

func TestCommandBuffer_Remove_Component_Should_Apply(t *testing.T) {
	t.Parallel()
	em := ecs.NewEntityManager()
	em.Add(ecs.NewEntity("enemy", []ecs.Component{&health{}, &mana{}}))
	buffer := ecs.NewCommandBuffer()
	buffer.RemoveComponent("enemy", maskMana)
	if err := buffer.Flush(em); err != nil {
		t.Fatalf("Flush should not fail, but got %v", err)
	}
	e := em.Get("enemy")
	if e.Get(maskMana) != nil {
		t.Error("mana should be gone after Flush")
	}
	if e.Get(maskHealth) == nil {
		t.Error("health should still be there after Flush")
	}
	if e.Mask() != maskHealth {
		t.Errorf("mask should be recalculated to %d, but got %d", maskHealth, e.Mask())
	}
}

func TestCommandBuffer_Flush_With_Nil_Entity_Should_Fail_And_Apply_Nothing(t *testing.T) {
	t.Parallel()
	em := ecs.NewEntityManager()
	em.Add(ecs.NewEntity("enemy", []ecs.Component{&health{}}))
	buffer := ecs.NewCommandBuffer()
	buffer.CreateEntity(ecs.NewEntity("drop", []ecs.Component{&loot{}}))
	buffer.CreateEntity(nil)
	buffer.DestroyEntity("enemy")
	if err := buffer.Flush(em); err == nil {
		t.Fatal("Flush should fail on a nil entity")
	}
	if em.Get("drop") != nil {
		t.Error("no command of the failed batch should be applied: drop should not be there")
	}
	if em.Get("enemy") == nil {
		t.Error("no command of the failed batch should be applied: enemy should still be there")
	}
	if buffer.Len() != 3 {
		t.Errorf("buffer should keep its commands on error, but holds %d", buffer.Len())
	}
}

func TestCommandBuffer_Flush_With_Nil_Component_Should_Fail_And_Apply_Nothing(t *testing.T) {
	t.Parallel()
	em := ecs.NewEntityManager()
	em.Add(ecs.NewEntity("enemy", []ecs.Component{&health{}}))
	buffer := ecs.NewCommandBuffer()
	buffer.AddComponent("enemy", nil)
	if err := buffer.Flush(em); err == nil {
		t.Fatal("Flush should fail on a nil component")
	}
	if got := len(em.Get("enemy").Components); got != 1 {
		t.Errorf("no command of the failed batch should be applied: enemy should carry 1 component, but got %d", got)
	}
}

func TestCommandBuffer_Flush_Empty_Should_Do_Nothing(t *testing.T) {
	t.Parallel()
	em := ecs.NewEntityManager()
	em.Add(ecs.NewEntity("enemy", []ecs.Component{&health{}}))
	buffer := ecs.NewCommandBuffer()
	if err := buffer.Flush(em); err != nil {
		t.Fatalf("Flush of an empty buffer should not fail, but got %v", err)
	}
	if got := len(em.Entities()); got != 1 {
		t.Errorf("manager should be unchanged, but holds %d entities", got)
	}
}

func TestCommandBuffer_System_Panic_Should_Leave_The_Manager_Untouched(t *testing.T) {
	t.Parallel()
	em := ecs.NewEntityManager()
	em.Add(ecs.NewEntity("enemy", []ecs.Component{&health{}}))
	buffer := ecs.NewCommandBuffer()
	bem := ecs.NewBufferedEntityManager(em, buffer)
	sm := ecs.NewSystemManager()
	sm.Add(&panickingSystem{})
	engine := ecs.NewDefaultEngine(bem, sm)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("the panic should propagate")
		}
		if em.Get("enemy") == nil {
			t.Error("enemy should still be there: the panicking frame never reached Commit")
		}
		if buffer.Len() != 1 {
			t.Errorf("buffer should still hold the queued destroy, but holds %d commands", buffer.Len())
		}
		buffer.Reset()
		if buffer.Len() != 0 {
			t.Errorf("buffer should be empty after Reset, but holds %d commands", buffer.Len())
		}
	}()
	engine.Tick()
}

// panickingSystem queues a destroy and then panics, the way a system fails in
// the middle of a frame.
type panickingSystem struct{}

func (s *panickingSystem) Setup() {}
func (s *panickingSystem) Process(em ecs.EntityManager) (state int) {
	em.Remove(em.Get("enemy"))
	panic("boom")
}
func (s *panickingSystem) Teardown() {}

// deathSystem destroys every entity whose health ran out and spawns a drop in
// its place. collectorSystem runs after it and collects every drop it sees.
// The two depend on each other through the manager: death produces what
// collector consumes, one frame later.
type deathSystem struct{}

func (s *deathSystem) Setup() {}
func (s *deathSystem) Process(em ecs.EntityManager) (state int) {
	for _, e := range em.FilterByMask(maskHealth) {
		if e.Get(maskHealth).(*health).points > 0 {
			continue
		}
		em.Remove(e)
		em.Add(ecs.NewEntity("drop-"+e.Id, []ecs.Component{&loot{}}))
	}
	return ecs.StateEngineContinue
}
func (s *deathSystem) Teardown() {}

type collectorSystem struct {
	collected []string
	seenDead  []string
}

func (s *collectorSystem) Setup() {}
func (s *collectorSystem) Process(em ecs.EntityManager) (state int) {
	// Record what the pre-commit view shows: dead enemies are still visible
	// this frame, their drops are not.
	for _, e := range em.FilterByMask(maskHealth) {
		if e.Get(maskHealth).(*health).points <= 0 {
			s.seenDead = append(s.seenDead, e.Id)
		}
	}
	for _, e := range em.FilterByMask(maskLoot) {
		s.collected = append(s.collected, e.Id)
		em.Remove(e)
	}
	return ecs.StateEngineContinue
}
func (s *collectorSystem) Teardown() {}

// recordingEntityManager wraps a DefaultEntityManager and records the order
// of the structural calls, so a test can assert the commit order.
type recordingEntityManager struct {
	inner *ecs.DefaultEntityManager
	log   []string
}

func (m *recordingEntityManager) Add(entities ...*ecs.Entity) {
	for _, e := range entities {
		m.log = append(m.log, "add:"+e.Id)
	}
	m.inner.Add(entities...)
}
func (m *recordingEntityManager) Entities() (entities []*ecs.Entity) {
	return m.inner.Entities()
}
func (m *recordingEntityManager) FilterByMask(mask uint64) (entities []*ecs.Entity) {
	return m.inner.FilterByMask(mask)
}
func (m *recordingEntityManager) FilterByNames(names ...string) (entities []*ecs.Entity) {
	return m.inner.FilterByNames(names...)
}
func (m *recordingEntityManager) Get(id string) (entity *ecs.Entity) {
	return m.inner.Get(id)
}
func (m *recordingEntityManager) Remove(entity *ecs.Entity) {
	if entity != nil {
		m.log = append(m.log, "remove:"+entity.Id)
	}
	m.inner.Remove(entity)
}

func TestBufferedEntityManager_Two_Dependent_Systems_Over_A_Fixed_Frame_Sequence(t *testing.T) {
	t.Parallel()
	recorder := &recordingEntityManager{inner: ecs.NewEntityManager()}
	recorder.inner.Add(
		ecs.NewEntity("enemy-a", []ecs.Component{&health{points: 0}}),
		ecs.NewEntity("enemy-b", []ecs.Component{&health{points: 1}}),
	)
	recorder.log = nil // the setup adds are not part of the commit
	bem := ecs.NewBufferedEntityManager(recorder, ecs.NewCommandBuffer())
	collector := &collectorSystem{}
	sm := ecs.NewSystemManager()
	sm.Add(&deathSystem{}, collector)
	engine := ecs.NewDefaultEngine(bem, sm)

	// Frame 1: death queues the destroy of enemy-a and the create of its
	// drop. Collector runs after death and must see the pre-commit view.
	engine.Tick()
	if len(collector.seenDead) != 1 || collector.seenDead[0] != "enemy-a" {
		t.Errorf("frame 1 should still see the dead enemy-a, but got %v", collector.seenDead)
	}
	if len(collector.collected) != 0 {
		t.Errorf("frame 1 should not see the queued drop yet, but collected %v", collector.collected)
	}
	if recorder.inner.Get("enemy-a") == nil {
		t.Error("frame 1 before Commit: enemy-a should still be in the manager")
	}
	if recorder.inner.Get("drop-enemy-a") != nil {
		t.Error("frame 1 before Commit: drop-enemy-a should not be in the manager yet")
	}
	if err := bem.Commit(); err != nil {
		t.Fatalf("Commit should not fail, but got %v", err)
	}
	wantLog := []string{"remove:enemy-a", "add:drop-enemy-a"}
	if len(recorder.log) != len(wantLog) {
		t.Fatalf("commit order should be %v, but got %v", wantLog, recorder.log)
	}
	for i := range wantLog {
		if recorder.log[i] != wantLog[i] {
			t.Fatalf("commit order should be %v, but got %v", wantLog, recorder.log)
		}
	}
	if recorder.inner.Get("enemy-a") != nil {
		t.Error("after Commit: enemy-a should be gone")
	}
	if recorder.inner.Get("drop-enemy-a") == nil {
		t.Error("after Commit: drop-enemy-a should be there")
	}
	if recorder.inner.Get("enemy-b") == nil {
		t.Error("after Commit: enemy-b should be untouched")
	}

	// Frame 2: collector now sees the drop and queues its removal. Death
	// finds no newly dead enemy.
	engine.Tick()
	if len(collector.collected) != 1 || collector.collected[0] != "drop-enemy-a" {
		t.Errorf("frame 2 should collect drop-enemy-a, but got %v", collector.collected)
	}
	if recorder.inner.Get("drop-enemy-a") == nil {
		t.Error("frame 2 before Commit: drop-enemy-a should still be in the manager")
	}
	if err := bem.Commit(); err != nil {
		t.Fatalf("Commit should not fail, but got %v", err)
	}
	if recorder.inner.Get("drop-enemy-a") != nil {
		t.Error("after Commit: drop-enemy-a should be collected and gone")
	}
	if got := len(recorder.inner.Entities()); got != 1 {
		t.Errorf("only enemy-b should remain, but got %d entities", got)
	}
}

func TestBufferedEntityManager_Remove_Nil_Should_Be_A_NoOp(t *testing.T) {
	t.Parallel()
	bem := ecs.NewBufferedEntityManager(ecs.NewEntityManager(), ecs.NewCommandBuffer())
	bem.Remove(bem.Get("ghost"))
	if bem.Buffer().Len() != 0 {
		t.Errorf("removing nil should queue nothing, but the buffer holds %d commands", bem.Buffer().Len())
	}
	if err := bem.Commit(); err != nil {
		t.Fatalf("Commit should not fail, but got %v", err)
	}
}
