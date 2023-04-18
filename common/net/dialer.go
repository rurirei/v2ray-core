package net

type DialFunc = func(string, string) (Conn, error)

type Dialer interface {
	Dial(string, string) (Conn, error)
}

type dialer struct {
	dialFunc     DialFunc
	lookupIPFunc LookupIPFunc
}

func NewDialer(dialFunc DialFunc, lookupIPFunc LookupIPFunc) Dialer {
	return &dialer{
		dialFunc:     dialFunc,
		lookupIPFunc: lookupIPFunc,
	}
}

func (d *dialer) Dial(network, address string) (Conn, error) {
	hostStr, portStr, err := SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	ips, err := d.lookupIPFunc(LookupIPOption.Network.This(), hostStr)
	if err != nil {
		return nil, err
	}

	errs := make([]error, 0, len(ips))

	for _, ip := range ips {
		address2 := JoinHostPort(ip.String(), portStr)

		conn, err := d.dialFunc(network, address2)
		if err == nil {
			return conn, nil
		}

		errs = append(errs, newError("dialFunc on %s (%s):", address, address2).WithError(err))
	}

	return nil, newError("all retries failed").WithError(errs)
}
