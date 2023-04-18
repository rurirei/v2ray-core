package dns

import (
	"golang.org/x/net/dns/dnsmessage"

	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/net"
)

type ipRecordComp struct {
	ip6, ip4 ipRecord
}

func (r ipRecordComp) getTTL() int64 {
	if r.ip6.ttl < r.ip4.ttl {
		return r.ip6.ttl
	}
	return r.ip4.ttl
}

type ipRecord struct {
	reqID uint16
	rCode dnsmessage.RCode
	ttl   int64
	ip    []net.IP
}

type ipRequest struct {
	fqdn    string
	reqType dnsmessage.Type
	msg     *dnsmessage.Message
}

// parseResponse parse DNS answers from the returned payload
func parseResponse(payload []byte) (ipRecord, error) {
	var parser dnsmessage.Parser
	header, err := parser.Start(payload)
	if err != nil {
		return ipRecord{}, newError("failed to parse DNS response").WithError(err)
	}
	if err := parser.SkipAllQuestions(); err != nil {
		return ipRecord{}, newError("failed to skip questions in DNS response").WithError(err)
	}

	rec := ipRecord{
		reqID: header.ID,
		rCode: header.RCode,
		ttl:   net.LookupIPOption.TTL,
	}

	for {
		answerHeader, err := parser.AnswerHeader()
		if err != nil {
			newError("failed to parse answer section for domain %s", answerHeader.Name.String()).WithError(err).AtDebug().Logging()
			break
		}

		if ttl := answerHeader.TTL; ttl > 0 {
			rec.ttl = int64(ttl)
		}

		switch answerHeader.Type {
		case dnsmessage.TypeAAAA:
			answer, err := parser.AAAAResource()
			if err != nil {
				newError("failed to parse AAAA rec for domain %s", answerHeader.Name).WithError(err).AtDebug().Logging()
				break
			}
			rec.ip = append(rec.ip, answer.AAAA[:])
		case dnsmessage.TypeA:
			answer, err := parser.AResource()
			if err != nil {
				newError("failed to parse A rec for domain %s", answerHeader.Name).WithError(err).AtDebug().Logging()
				break
			}
			rec.ip = append(rec.ip, answer.A[:])
		default:
			if err := parser.SkipAnswer(); err != nil {
				newError("failed to skip answer").WithError(err).AtDebug().Logging()
				break
			}
		}
	}

	return rec, nil
}

func buildRequest(fqdn string, reqIDGen func() uint16) []ipRequest {
	req := make([]ipRequest, 0, 2)

	// ipv6
	func() {
		q6 := dnsmessage.Question{
			Name:  dnsmessage.MustNewName(fqdn),
			Type:  dnsmessage.TypeAAAA,
			Class: dnsmessage.ClassINET,
		}
		msg := new(dnsmessage.Message)
		msg.Header.ID = reqIDGen()
		msg.Header.RecursionDesired = true
		msg.Questions = []dnsmessage.Question{q6}
		req = append(req, ipRequest{
			fqdn:    fqdn,
			reqType: dnsmessage.TypeAAAA,
			msg:     msg,
		})
	}()

	// ipv4
	func() {
		q4 := dnsmessage.Question{
			Name:  dnsmessage.MustNewName(fqdn),
			Type:  dnsmessage.TypeA,
			Class: dnsmessage.ClassINET,
		}
		msg := new(dnsmessage.Message)
		msg.Header.ID = reqIDGen()
		msg.Header.RecursionDesired = true
		msg.Questions = []dnsmessage.Question{q4}
		req = append(req, ipRequest{
			fqdn:    fqdn,
			reqType: dnsmessage.TypeA,
			msg:     msg,
		})
	}()

	return req
}

func packMessage(msg *dnsmessage.Message) (*buffer.Buffer, error) {
	buf := buffer.New()
	rawBytes := buf.Extend(buffer.Size)

	packed, err := msg.AppendPack(rawBytes[:0])
	if err != nil {
		defer buf.Release()
		return nil, err
	}

	buf.Resize(0, len(packed))
	return buf, nil
}
