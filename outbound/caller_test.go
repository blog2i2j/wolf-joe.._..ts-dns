package outbound

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey"
	"github.com/miekg/dns"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/wolf-joe/ts-dns/core/utils"
	"github.com/wolf-joe/ts-dns/core/utils/mock"
	"golang.org/x/net/proxy"
)

var dialer, _ = proxy.SOCKS5("tcp", "", nil, proxy.Direct)

func assertFail(t *testing.T, val *dns.Msg, err error) {
	assert.Nil(t, val)
	assert.NotNil(t, err)
}
func assertSuccess(t *testing.T, val *dns.Msg, err error) {
	assert.NotNil(t, val)
	assert.Nil(t, err)
}

func MockMethodSeq(target interface{}, methodName string, outputs []gomonkey.Params) *gomonkey.Patches {
	var cells []gomonkey.OutputCell
	for _, output := range outputs {
		cells = append(cells, gomonkey.OutputCell{Values: output})
	}
	return gomonkey.ApplyMethodSeq(reflect.TypeOf(target), methodName, cells)
}

func TestDNSCaller(t *testing.T) {
	req := &dns.Msg{}

	caller := NewDNSCaller("", "", nil)
	// 不使用代理，mock掉Exchange
	p := MockMethodSeq(caller.client, "Exchange", []gomonkey.Params{
		{nil, time.Second, fmt.Errorf("err")},
		{&dns.Msg{}, time.Second, nil},
	})
	// exchange调用失败
	r, err := caller.Call(req)
	assertFail(t, r, err)
	// exchange调用成功
	r, err = caller.Call(req)
	assertSuccess(t, r, err)

	caller.Exit()
	_ = caller.String()

	caller = NewDoTCaller("", "", dialer)
	// 使用代理，mock掉Dial、WriteMsg、ReadMsg
	p1 := MockMethodSeq(caller.proxy, "Dial", []gomonkey.Params{
		{nil, fmt.Errorf("err")},
		{&net.TCPConn{}, nil}, {&net.TCPConn{}, nil}, {&net.TCPConn{}, nil},
	})
	p2 := MockMethodSeq(caller.conn, "WriteMsg", []gomonkey.Params{
		{fmt.Errorf("err")}, {nil}, {nil},
	})
	p3 := MockMethodSeq(caller.conn, "ReadMsg", []gomonkey.Params{
		{nil, fmt.Errorf("err")}, {&dns.Msg{}, nil},
	})
	defer func() { p.Reset(); p1.Reset(); p2.Reset(); p3.Reset() }()
	// Dial失败
	r, err = caller.Call(req)
	assertFail(t, r, err)
	// Dial成功，但WriteMsg失败
	r, err = caller.Call(req)
	assertFail(t, r, err)
	// Dial、WriteMsg成功，但ReadMsg失败
	r, err = caller.Call(req)
	assertFail(t, r, err)
	// Dial、WriteMsg、ReadMsg都成功
	r, err = caller.Call(req)
	assertSuccess(t, r, err)

	caller.Exit()
	_ = caller.String()
}

func wrapperHandler(serveDNS func(req *dns.Msg) *dns.Msg) dns.HandlerFunc {
	handlerFunc := func(writer dns.ResponseWriter, req *dns.Msg) {
		defer func() { _ = writer.Close() }()
		resp := serveDNS(req)
		if resp != nil {
			resp.SetReply(req)
		}
		_ = writer.WriteMsg(resp)
	}
	return handlerFunc
}

func TestDoHCallerV2(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	ctx := utils.NewCtx(nil, 0xffff)

	// 测试解析url失败的case
	caller, err := NewDoHCallerV2(ctx, "\n", nil)
	assert.NotNil(t, err)
	caller, err = NewDoHCallerV2(ctx, "abc", nil)
	assert.NotNil(t, err)
	caller, err = NewDoHCallerV2(ctx, "https://abc::/", nil)
	assert.NotNil(t, err)

	url := "https://dns.alidns.com/dns-query"

	// 测试run和stop
	caller, err = NewDoHCallerV2(ctx, url, nil)
	assert.Nil(t, err)
	caller.Exit()
	time.Sleep(time.Millisecond * 100) // wait exit
	go func(c *DoHCallerV2) {
		time.Sleep(time.Millisecond * 100)
		c.Exit()
	}(caller)
	caller.run(time.After(0), time.Second)

	req := &dns.Msg{
		MsgHdr:   dns.MsgHdr{Id: 0xffff, RecursionDesired: true, AuthenticatedData: true},
		Question: []dns.Question{{Name: "BAIDU.COM.", Qtype: dns.TypeA, Qclass: dns.ClassINET}},
	}

	// 测试解析超时的case
	resolver := wrapperHandler(func(req *dns.Msg) *dns.Msg {
		time.Sleep(time.Second * 3)
		return nil
	})
	caller, err = NewDoHCallerV2(ctx, url, nil)
	assert.Nil(t, err)
	caller.SetResolver(resolver)
	_, err = caller.Call(req)
	assert.NotNil(t, err) // timeout
	caller.Exit()

	// 测试回环解析
	recReq := &dns.Msg{
		MsgHdr:   dns.MsgHdr{Id: 0xffff, RecursionDesired: true, AuthenticatedData: true},
		Question: []dns.Question{{Name: "DNS.ALIDNS.COM.", Qtype: dns.TypeA, Qclass: dns.ClassINET}},
	}
	caller, err = NewDoHCallerV2(ctx, url, nil)
	assert.Nil(t, err)
	caller.SetResolver(resolver)
	_, err = caller.Call(recReq)
	assert.NotNil(t, err) // timeout
	caller.Exit()

	mocker := mock.Mocker{}
	defer mocker.Reset()
	httpReq := &http.Request{Header: map[string][]string{}}
	mocker.MethodSeq(req, "PackBuffer", []gomonkey.Params{
		{nil, fmt.Errorf("err")}, {[]byte{1}, nil}, {[]byte{1}, nil},
		{[]byte{1}, nil}, {[]byte{1}, nil}, {[]byte{1}, nil},
	})
	mocker.FuncSeq(http.NewRequest, []gomonkey.Params{
		{nil, fmt.Errorf("err")}, {httpReq, nil}, {httpReq, nil},
		{httpReq, nil}, {httpReq, nil},
	})
	mocker.MethodSeq(&http.Client{}, "Do", []gomonkey.Params{
		{nil, fmt.Errorf("err")}, {&http.Response{Body: &net.TCPConn{}}, nil},
		{&http.Response{Body: &net.TCPConn{}}, nil},
		{&http.Response{Body: &net.TCPConn{}}, nil},
	})
	mocker.FuncSeq(ioutil.ReadAll, []gomonkey.Params{
		{nil, fmt.Errorf("err")}, {make([]byte, 1), nil},
		{make([]byte, 12), nil},
	})

	// 测试正常解析的case
	resolver = wrapperHandler(func(req *dns.Msg) *dns.Msg {
		return &dns.Msg{Answer: []dns.RR{
			&dns.A{A: net.IPv4(223, 5, 5, 5)},
		}}
	})
	caller, err = NewDoHCallerV2(ctx, url, nil)
	assert.Nil(t, err)
	caller.SetResolver(resolver)
	// Pack失败
	resp, err := caller.Call(req)
	assert.NotNil(t, err)
	// Pack成功，但NewRequest失败
	resp, err = caller.Call(req)
	assert.NotNil(t, err)
	// Pack、NewRequest成功，但Do失败
	resp, err = caller.Call(req)
	assert.NotNil(t, err)
	// Pack、NewRequest、Do成功，但ReadAll失败
	resp, err = caller.Call(req)
	assert.NotNil(t, err)
	// Pack、NewRequest、Do、ReadAll成功，但Unpack失败
	resp, err = caller.Call(req)
	assert.NotNil(t, err)
	// Pack、NewRequest、Do、ReadAll、Unpack成功
	resp, err = caller.Call(req)
	assert.Nil(t, err)
	assert.NotNil(t, resp)

	// 测试DialContext
	if len(caller.clients) > 0 {
		trans := caller.clients[0].Transport.(*http.Transport)
		_, _ = trans.DialContext(nil, "", "")
	}
	caller.Exit()
	_ = caller.String()

}
