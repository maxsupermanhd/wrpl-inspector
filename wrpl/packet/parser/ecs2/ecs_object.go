package packetecs2

const ENTITY_INDEX_BITS = 22
const ENTITY_INDEX_MASK = (1 << ENTITY_INDEX_BITS) - 1
const ECS_INVALID_ENTITY_ID_VAL EntityID = 0

type EntityID uint32

func (id EntityID) Index() uint32 { // we probably only need index
	return uint32(id & ENTITY_INDEX_MASK)
}

type Component struct {
	Value any           // actual data, I think this will ensure the Component owns?
	Type  ComponentHash // what type
}
type NamedComponent struct {
	Comp *Component
	Name string
}
type Object struct {
	Components []NamedComponent
}

func (o *Object) lookupComponent(name string) *Component {
	for _, component := range o.Components {
		if component.Name == name {
			return component.Comp
		}
	}
	return nil
}

func (o *Object) AddComponent(comp *Component, name string) {
	var named NamedComponent
	named.Name = name
	named.Comp = comp
	o.Components = append(o.Components, named)
}

func (o *Object) GetData(name string) (any, bool) {
	comp := o.lookupComponent(name)
	if comp == nil {
		return nil, false
	}
	return comp.Value, true
}

func (o *Object) GetDataPtr(name string) (any, bool) {
	comp := o.lookupComponent(name)
	if comp == nil {
		return nil, false
	}
	return &comp.Value, true
}

func GetObjectData[T any](o *Object, name string) (T, bool) {
	val, ok := o.GetData(name)
	if !ok {
		var temp T
		return temp, false
	}
	ret, succ := val.(T)
	if !succ {
		var temp T
		return temp, false
	}
	return ret, true
}

func GetObjectDataPtr[T any](o *Object, name string) (*T, bool) {
	val, ok := o.GetDataPtr(name)
	if !ok {
		return nil, false
	}
	ret, succ := val.(*T)
	if !succ {
		return nil, false
	}
	return ret, true
}

type Entity struct {
	Template string
	Data     Object
}
