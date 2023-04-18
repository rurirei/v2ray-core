package drain

import (
	"v2ray.com/core/common/dice"
	"v2ray.com/core/common/io"
)

type BehaviorSeedLimitedDrainer struct {
	DrainSize int
}

func NewBehaviorSeedLimitedDrainer(behaviorSeed int64, drainFoundation, maxBaseDrainSize, maxRandDrain int) (Drainer, error) {
	behaviorRand := dice.NewDeterministicDice(behaviorSeed)
	BaseDrainSize := behaviorRand.Roll(maxBaseDrainSize)
	RandDrainMax := behaviorRand.Roll(maxRandDrain) + 1
	RandDrainRolled := dice.Roll(RandDrainMax)
	DrainSize := drainFoundation + BaseDrainSize + RandDrainRolled
	return &BehaviorSeedLimitedDrainer{DrainSize: DrainSize}, nil
}

func (d *BehaviorSeedLimitedDrainer) AcknowledgeReceive(size int) {
	d.DrainSize -= size
}

func (d *BehaviorSeedLimitedDrainer) Drain(reader io.Reader) error {
	if d.DrainSize > 0 {
		err := drainReadN(reader, d.DrainSize)
		if err == nil {
			return newError("drained connection")
		}
		return newError("unable to drain connection").WithError(err)
	}
	return nil
}

func drainReadN(reader io.Reader, n int) error {
	_, err := io.Discard(io.LimitReader(reader, int64(n)))
	return err
}

func WithError(drainer Drainer, reader io.Reader, err error) error {
	drainErr := drainer.Drain(reader)
	if drainErr == nil {
		return err
	}
	return newError(drainErr.Error()).WithError(err)
}

type NopDrainer struct{}

func (n NopDrainer) AcknowledgeReceive(_ int) {
}

func (n NopDrainer) Drain(_ io.Reader) error {
	return nil
}

func NewNopDrainer() Drainer {
	return &NopDrainer{}
}