package packetecs2

type EntityManager struct {
	Entities   map[uint32]*Entity
	Uid_lookup map[int32]*Entity
}

func (mgr *EntityManager) AddEntity(eid EntityID, entity *Entity) {
	mgr.Entities[eid.Index()] = entity
	val, ok := GetObjectData[int32](&entity.Data, "uid")
	if ok {
		mgr.Uid_lookup[val] = entity
	}
}

func (mgr *EntityManager) GetEntity(eid EntityID) (*Entity, bool) { // TODO, maybe return an err instead?
	val, ok := mgr.Entities[eid.Index()]
	return val, ok
}

func (mgr *EntityManager) GetEntityByUid(uid int32) (*Entity, bool) {
	val, ok := mgr.Uid_lookup[uid]
	return val, ok
}
