package ecs

import "fmt"

// The kinds of structural change a CommandBuffer can queue.
const (
	opCreateEntity = iota
	opDestroyEntity
	opAddComponent
	opRemoveComponent
)

// command is one queued structural change. Which fields are set depends on op.
type command struct {
	op        int
	entity    *Entity   // opCreateEntity
	id        string    // opDestroyEntity, opAddComponent, opRemoveComponent
	component Component // opAddComponent
	mask      uint64    // opRemoveComponent
}

// CommandBuffer queues structural changes — creating and destroying entities,
// adding and removing components — while the systems of a frame run, and
// applies them all at once when the frame ends. Until Flush runs, the
// EntityManager is untouched, so every system of the frame queries the same
// pre-commit view: an enemy that dies and spawns a drop in one frame is not
// seen again — nor its drop seen early — by the systems that come after.
//
// The zero value is ready to use. Like the rest of the package, a
// CommandBuffer is not safe for concurrent use.
type CommandBuffer struct {
	commands []command
}

// CreateEntity queues the entity to be added to the manager on Flush.
func (b *CommandBuffer) CreateEntity(entity *Entity) {
	b.commands = append(b.commands, command{op: opCreateEntity, entity: entity})
}

// DestroyEntity queues the entity with the given id to be removed from the
// manager on Flush.
func (b *CommandBuffer) DestroyEntity(id string) {
	b.commands = append(b.commands, command{op: opDestroyEntity, id: id})
}

// AddComponent queues the component to be added to the entity with the given
// id on Flush. The entity's own Add decides what happens when it already
// carries a component with the same mask.
func (b *CommandBuffer) AddComponent(id string, component Component) {
	b.commands = append(b.commands, command{op: opAddComponent, id: id, component: component})
}

// RemoveComponent queues the component with the given mask to be removed from
// the entity with the given id on Flush.
func (b *CommandBuffer) RemoveComponent(id string, mask uint64) {
	b.commands = append(b.commands, command{op: opRemoveComponent, id: id, mask: mask})
}

// Len returns the number of queued commands.
func (b *CommandBuffer) Len() int {
	return len(b.commands)
}

// Reset discards every queued command without applying any. It is the way out
// after a frame went wrong — a system panicked, say — when the queued changes
// should never reach the manager.
func (b *CommandBuffer) Reset() {
	b.commands = b.commands[:0]
}

// Flush applies every queued command to em, in the order the commands were
// queued, and empties the buffer, so a flushed buffer commits nothing twice.
// The rules for conflicts are:
//
//   - CreateEntity is dropped when the id is already taken — by an entity em
//     holds or by an earlier create in the same batch. The first claim wins.
//   - DestroyEntity of an id em does not hold is a no-op, so a duplicate
//     destroy, or a destroy of an unknown entity, is safe. A destroy after a
//     create of the same id removes the just-created entity: the two cancel.
//   - AddComponent and RemoveComponent on an entity em does not hold —
//     unknown, or already destroyed in the same batch — are no-ops.
//
// Flush validates the whole batch before applying anything: a nil entity or
// a nil component fails the Flush with an error and em is left untouched,
// never half-updated. On error the buffer keeps its commands, so the caller
// can inspect them or Reset.
//
// Flush never panics and never recovers one: a system that panics while
// queueing propagates as usual, and the manager keeps its pre-frame state
// because Flush was never reached.
func (b *CommandBuffer) Flush(em EntityManager) error {
	for i, c := range b.commands {
		switch c.op {
		case opCreateEntity:
			if c.entity == nil {
				return fmt.Errorf("ecs: command %d creates a nil entity", i)
			}
		case opAddComponent:
			if c.component == nil {
				return fmt.Errorf("ecs: command %d adds a nil component to %q", i, c.id)
			}
		}
	}
	for _, c := range b.commands {
		switch c.op {
		case opCreateEntity:
			if em.Get(c.entity.Id) != nil {
				continue
			}
			em.Add(c.entity)
		case opDestroyEntity:
			em.Remove(em.Get(c.id))
		case opAddComponent:
			if e := em.Get(c.id); e != nil {
				e.Add(c.component)
			}
		case opRemoveComponent:
			if e := em.Get(c.id); e != nil {
				e.Remove(c.mask)
			}
		}
	}
	b.Reset()
	return nil
}

// NewCommandBuffer creates a new CommandBuffer and returns its address.
func NewCommandBuffer() *CommandBuffer {
	return &CommandBuffer{
		commands: []command{},
	}
}
