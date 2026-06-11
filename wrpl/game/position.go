package game

type SpaceTime struct {
	Time uint32
	X    float64
	Y    float64
	Z    float64
}

type PositionByUidProvider interface {
	GetPositionByUid(uid uint16, t uint32) *SpaceTime
}

type PositionByEidProvider interface {
	GetPositionByEid(eid uint32, t uint32) *SpaceTime
}

type PositionByEidxProvider interface {
	GetPositionByEidx(eidx uint32, t uint32) *SpaceTime
}

type UidToEidResolver interface {
	UidToEid(uid uint16) *uint32
}

type SimplePositionCoordinator struct {
	Providers []PositionByEidProvider
	Resolver  UidToEidResolver
}

func NewSimplePositionCoordinator(providers []PositionByEidProvider, resolver UidToEidResolver) *SimplePositionCoordinator {
	return &SimplePositionCoordinator{
		Providers: providers,
		Resolver:  resolver,
	}
}

func (c *SimplePositionCoordinator) GetPositionByEid(eid uint32, t uint32) *SpaceTime {
	var ret *SpaceTime
	for _, provider := range c.Providers {
		pos := provider.GetPositionByEid(eid, t)
		if pos == nil {
			continue
		}
		if ret == nil {
			ret = pos
		} else {
			if pos.Time > ret.Time {
				ret = pos
			}
		}
	}
	return ret
}

func (c *SimplePositionCoordinator) GetPositionByUid(uid uint16, t uint32) *SpaceTime {
	eid := c.Resolver.UidToEid(uid)
	if eid == nil {
		return nil
	}
	return c.GetPositionByEid(*eid, t)
}
