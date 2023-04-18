package dns

import (
	"golang.org/x/net/dns/dnsmessage"

	"v2ray.com/core/common"
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol/dns"
	"v2ray.com/core/common/session"
	"v2ray.com/core/common/task"
	"v2ray.com/core/proxy"
	"v2ray.com/core/transport"
	"v2ray.com/core/transport/internet"
)

type LookupIPConditionFunc = func(string, string, session.Lookup) ([]net.IP, error)

type ClientSetting struct {
	LookupIPConditionFunc LookupIPConditionFunc
}

type client struct {
	lookupIPConditionFunc LookupIPConditionFunc
}

func NewClient(setting ClientSetting) proxy.Client {
	return &client{
		lookupIPConditionFunc: setting.LookupIPConditionFunc,
	}
}

func (c *client) Process(content session.Content, address net.Address, link transport.Link, _ internet.DialTCPFunc, _ internet.DialUDPFunc) error {
	defer func() {
		_ = link.Writer.Close()
	}()

	lookupIPFunc := func(network string, host string) ([]net.IP, error) {
		ib, _ := content.GetInbound()

		return c.lookupIPConditionFunc(network, host, session.Lookup{
			Domain:     host,
			InboundTag: ib.Tag,
		})
	}

	msgWriter, msgReader, err := func(address net.Address) (dns.MessageWriter, dns.MessageReader, error) {
		ib, _ := content.GetInbound()

		switch ib.Source.Network {
		case net.Network_TCP:
			return dns.NewTCPWriter(link.Writer), dns.NewTCPReader(buffer.NewBufferedReader(link.Reader)), nil
		case net.Network_UDP:
			return dns.NewUDPWriter(link.Writer), dns.NewUDPReader(link.Reader), nil
		default:
			return nil, nil, common.ErrUnknownNetwork
		}
	}(address)
	if err != nil {
		return err
	}

	requestDone := func() error {
		for {
			b, err := msgReader.ReadMessage()
			if err != nil {
				return err
			}

			req, err := parseIPQuery(b.Bytes())
			if err != nil {
				return err
			}

			if err := handleIPQuery(msgWriter, lookupIPFunc, req); err != nil {
				return err
			}
		}
	}

	if errs := task.Parallel(requestDone); len(errs) > 0 {
		return newError("connection ends").WithError(errs)
	}
	return nil
}

func handleIPQuery(writer dns.MessageWriter, lookupIPFunc net.LookupIPFunc, req ipRequest) error {
	pack := func(ips []net.IP) error {
		buf := buffer.New()
		rawBytes := buf.Extend(buffer.Size)

		builder := dnsmessage.NewBuilder(rawBytes[:0], dnsmessage.Header{
			ID:                 req.reqID,
			RCode:              dnsmessage.RCodeSuccess,
			RecursionAvailable: true,
			RecursionDesired:   true,
			Response:           true,
		})
		builder.EnableCompression()
		if err := builder.StartQuestions(); err != nil {
			defer buf.Release()
			return err
		}
		if err := builder.Question(dnsmessage.Question{
			Name:  req.fqdn,
			Class: dnsmessage.ClassINET,
			Type:  req.reqType,
		}); err != nil {
			defer buf.Release()
			return err
		}
		if err := builder.StartAnswers(); err != nil {
			defer buf.Release()
			return err
		}

		rHeader := dnsmessage.ResourceHeader{
			Name:  req.fqdn,
			Class: dnsmessage.ClassINET,
			TTL:   uint32(net.LookupIPOption.TTL),
		}
		for _, ip := range ips {
			switch len(ip) {
			case net.IPv6len:
				var r dnsmessage.AAAAResource
				copy(r.AAAA[:], ip)
				if err := builder.AAAAResource(rHeader, r); err != nil {
					buf.Release()
					return err
				}
			case net.IPv4len:
				var r dnsmessage.AResource
				copy(r.A[:], ip)
				if err := builder.AResource(rHeader, r); err != nil {
					buf.Release()
					return err
				}
			default:
				buf.Release()
				return newError("unknown ip length")
			}
		}

		msgBytes, err := builder.Finish()
		if err != nil {
			defer buf.Release()
			return err
		}

		buf.Resize(0, len(msgBytes))
		return writer.WriteMessage(buf)
	}

	ips, err := lookupIPFunc(net.LookupIPOption.Network.This(), dns.TrimFqdn(req.fqdn.String()))
	if err != nil {
		return err
	}

	return pack(ips)
}

type ipRequest struct {
	fqdn    dnsmessage.Name
	reqType dnsmessage.Type
	reqID   uint16
}

func parseIPQuery(b []byte) (ipRequest, error) {
	var parser dnsmessage.Parser

	header, err := parser.Start(b)
	if err != nil {
		return ipRequest{}, err
	}

	question, err := parser.Question()
	if err != nil {
		return ipRequest{}, err
	}

	switch question.Type {
	case dnsmessage.TypeAAAA, dnsmessage.TypeA:
		return ipRequest{
			fqdn:    question.Name,
			reqType: question.Type,
			reqID:   header.ID,
		}, nil
	default:
		return ipRequest{}, newError("not ip query")
	}
}
