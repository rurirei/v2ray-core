package router

type Rule struct {
	Condition   Condition
	OutboundTag string
}
