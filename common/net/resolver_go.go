package net

import (
	"context"
)

func CGOResolverFunc() LookupIPFunc {
	r := &GoResolver{
		PreferGo: false,
	}
	return func(network, address string) ([]IP, error) {
		return r.LookupIP(context.Background(), network, address)
	}
}

func GOResolverFunc() LookupIPFunc {
	r := &GoResolver{
		PreferGo:     true,
		StrictErrors: true,
	}
	return func(network, address string) ([]IP, error) {
		return r.LookupIP(context.Background(), network, address)
	}
}

func GOResolverDialerFunc(dial DialFunc) LookupIPFunc {
	r := &GoResolver{
		PreferGo:     true,
		StrictErrors: true,
		Dial: func(_ context.Context, network, address string) (Conn, error) {
			return dial(network, address)
		},
	}
	return func(network, address string) ([]IP, error) {
		return r.LookupIP(context.Background(), network, address)
	}
}

func EmptyResolverFunc() LookupIPFunc {
	dial := func(network, address string) (Conn, error) {
		return nil, newError("the request is at empty resolver: %s:%s", network, address)
	}
	return GOResolverDialerFunc(dial)
}
