package ecs

var _ EntityManager = (*BufferedEntityManager)(nil)

// BufferedEntityManager is an EntityManager that defers structural changes
// through a CommandBuffer. Add and Remove queue a create or a destroy instead
// of touching the wrapped manager, and AddComponent and RemoveComponent queue
// the component change. Every read — Entities, the filters, Get — passes
// straight through to the wrapped manager, so during a frame all systems see
// the same pre-commit view. Commit applies the queue when the frame ends.
//
// Hand it to the engine where the plain manager went: systems keep compiling
// and keep reading as before, only their writes turn into commands. The
// wrapped manager used directly keeps its immediate behaviour, so code
// outside the frame — setup, tools, tests — is unaffected.
type BufferedEntityManager struct {
	inner  EntityManager
	buffer *CommandBuffer
}

// Add queues a create for each entity. The entities appear in the manager on
// Commit, not before.
func (m *BufferedEntityManager) Add(entities ...*Entity) {
	for _, e := range entities {
		m.buffer.CreateEntity(e)
	}
}

// Remove queues a destroy for the entity's id. It does nothing when entity is
// nil, so Remove(Get(id)) stays safe for an id that is not there.
func (m *BufferedEntityManager) Remove(entity *Entity) {
	if entity == nil {
		return
	}
	m.buffer.DestroyEntity(entity.Id)
}

// AddComponent queues the component to be added to the entity with the given
// id on Commit.
func (m *BufferedEntityManager) AddComponent(id string, component Component) {
	m.buffer.AddComponent(id, component)
}

// RemoveComponent queues the component with the given mask to be removed from
// the entity with the given id on Commit.
func (m *BufferedEntityManager) RemoveComponent(id string, mask uint64) {
	m.buffer.RemoveComponent(id, mask)
}

// Entities returns the wrapped manager's entities: the pre-commit view.
func (m *BufferedEntityManager) Entities() (entities []*Entity) {
	return m.inner.Entities()
}

// FilterByMask returns every entity of the pre-commit view that carries all
// the components in mask.
func (m *BufferedEntityManager) FilterByMask(mask uint64) (entities []*Entity) {
	return m.inner.FilterByMask(mask)
}

// FilterByNames returns every entity of the pre-commit view that carries a
// component for each of the given names.
func (m *BufferedEntityManager) FilterByNames(names ...string) (entities []*Entity) {
	return m.inner.FilterByNames(names...)
}

// Get returns the entity with the given Id from the pre-commit view, or nil
// when there is none.
func (m *BufferedEntityManager) Get(id string) (entity *Entity) {
	return m.inner.Get(id)
}

// Commit applies every queued command to the wrapped manager. It is the
// frame-end step: call it once per pass, after the last system ran. Its
// contract is the buffer's Flush contract — a failed Commit applies nothing.
func (m *BufferedEntityManager) Commit() error {
	return m.buffer.Flush(m.inner)
}

// Buffer returns the CommandBuffer the manager queues into, so the frame
// owner can inspect its length or Reset it after a system panicked.
func (m *BufferedEntityManager) Buffer() *CommandBuffer {
	return m.buffer
}

// NewBufferedEntityManager creates a new BufferedEntityManager wrapping inner
// and queueing into buffer, and returns its address.
func NewBufferedEntityManager(inner EntityManager, buffer *CommandBuffer) *BufferedEntityManager {
	return &BufferedEntityManager{
		inner:  inner,
		buffer: buffer,
	}
}
