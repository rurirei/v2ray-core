package cache

type RangeFunc = func(interface{}, interface{}) bool

type Pool interface {
	Get(interface{}) (interface{}, bool)

	Set(interface{}, interface{})
	SetExpire(interface{}, interface{}, int64)

	Delete(interface{})

	Range(RangeFunc)

	Close() error
}
