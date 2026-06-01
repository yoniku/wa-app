package app

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	waappv1 "github.com/byte-v-forge/wa-app/gen/go/byte/v/forge/waapp/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultChatdHost       = "g.whatsapp.net"
	defaultChatdPort       = 443
	defaultChatdMaxFrame   = 4 << 20
	defaultChatdReadWindow = 15 * time.Second
)

type chatdClientConfig struct {
	Host          string
	Port          int
	TLS           bool
	RoutingInfo   string
	ProxyURL      string
	InsecureTLS   bool
	Timeout       time.Duration
	MaxFrameBytes int
}

type chatdClient struct {
	cfg   chatdClientConfig
	codec *binaryNodeCodec
}

func newChatdClient(cfg chatdClientConfig) *chatdClient {
	if cfg.Host == "" {
		cfg.Host = defaultChatdHost
	}
	if cfg.Port == 0 {
		cfg.Port = defaultChatdPort
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultChatdReadWindow
	}
	if cfg.MaxFrameBytes <= 0 {
		cfg.MaxFrameBytes = defaultChatdMaxFrame
	}
	return &chatdClient{cfg: cfg, codec: newBinaryNodeCodec()}
}

func (c *chatdClient) receiveBatch(ctx context.Context, state nativeState, input EngineMessageInput, appVersion string, ids IDGenerator, now time.Time) ([]*waappv1.InboundMessage, []chatdEncPayload, error) {
	state.ChatStatic = ensureChatStatic(state.ChatStatic)
	privateKey, err := state.ChatStatic.privateBytes()
	if err != nil {
		return nil, nil, err
	}
	publicKey, err := state.ChatStatic.publicBytes()
	if err != nil {
		return nil, nil, err
	}
	identity, err := resolveLoginIdentity(input.RegisteredIdentityID, state)
	if err != nil {
		return nil, nil, err
	}
	conn, err := c.dial(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(c.cfg.Timeout))
	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	routingInfo, err := decodeRoutingInfo(c.cfg.RoutingInfo)
	if err != nil {
		return nil, nil, err
	}
	loginPayload := defaultLoginPayload(identity, state, appVersion)
	keys, err := doNoiseHandshake(rw, privateKey, publicKey, loginPayload, routingInfo, c.cfg.MaxFrameBytes)
	if err != nil {
		return nil, nil, err
	}
	transport := chatdTransport{rw: rw, keys: keys, codec: c.codec, maxFrameBytes: c.cfg.MaxFrameBytes}
	_ = transport.sendNode(buildPingNode())
	maxMessages := input.MaxMessages
	if maxMessages <= 0 {
		maxMessages = 10
	}
	deadline := waitDeadline(input.WaitTimeout)
	messages := []*waappv1.InboundMessage{}
	payloads := []chatdEncPayload{}
	for len(messages) < maxMessages && time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(time.Until(deadline)))
		node, err := transport.readNode()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				break
			}
			if len(messages) > 0 {
				break
			}
			return nil, nil, err
		}
		if ack, ok := buildAckForNode(node); ok {
			_ = transport.sendNode(ack)
		}
		encs := iterEncPayloads(node)
		if len(encs) == 0 && node.Tag != "message" && node.Tag != "notification" {
			continue
		}
		if len(encs) == 0 {
			messages = append(messages, &waappv1.InboundMessage{MessageId: ids.NewID("wamsg_"), MessageSessionId: input.MessageSessionID, Kind: inboundKind(node.Tag), EncryptionState: waappv1.MessageEncryptionState_MESSAGE_ENCRYPTION_STATE_PLAINTEXT, AckStatus: ackStatusForNode(node), SenderRef: firstNonEmpty(node.Attrs["participant"], node.Attrs["from"]), PayloadRef: "node:" + redacted(nodePayloadSummary(node)), ReceivedAt: timestamppb.New(now)})
			continue
		}
		for _, enc := range encs {
			payloadRef := payloadRefForEnc(input.MessageSessionID, enc.Payload)
			payloads = append(payloads, enc)
			messages = append(messages, &waappv1.InboundMessage{MessageId: ids.NewID("wamsg_"), MessageSessionId: input.MessageSessionID, Kind: inboundKind(node.Tag), EncryptionState: waappv1.MessageEncryptionState_MESSAGE_ENCRYPTION_STATE_ENCRYPTED, AckStatus: ackStatusForNode(node), SenderRef: enc.Sender, PayloadRef: payloadRef, ReceivedAt: timestamppb.New(now)})
			if len(messages) >= maxMessages {
				break
			}
		}
	}
	return messages, payloads, nil
}

func (c *chatdClient) checkLoginState(ctx context.Context, state nativeState, input EngineLoginCheckInput, appVersion string) error {
	state.ChatStatic = ensureChatStatic(state.ChatStatic)
	privateKey, err := state.ChatStatic.privateBytes()
	if err != nil {
		return err
	}
	publicKey, err := state.ChatStatic.publicBytes()
	if err != nil {
		return err
	}
	identity, err := resolveLoginIdentity(input.RegisteredIdentityID, state)
	if err != nil {
		return err
	}
	conn, err := c.dial(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(c.cfg.Timeout))
	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	routingInfo, err := decodeRoutingInfo(c.cfg.RoutingInfo)
	if err != nil {
		return err
	}
	loginPayload := passiveLoginCheckPayload(identity, state, appVersion)
	keys, err := doNoiseHandshake(rw, privateKey, publicKey, loginPayload, routingInfo, c.cfg.MaxFrameBytes)
	if err != nil {
		return err
	}
	transport := chatdTransport{rw: rw, keys: keys, codec: c.codec, maxFrameBytes: c.cfg.MaxFrameBytes}
	if err := transport.sendNode(buildPingNode()); err != nil {
		return err
	}
	_ = conn.SetReadDeadline(time.Now().Add(minDuration(2*time.Second, c.cfg.Timeout)))
	if _, err := transport.readNode(); err != nil {
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			return nil
		}
		return err
	}
	return nil
}

func ensureChatStatic(key nativeCurveKeyPair) nativeCurveKeyPair {
	if key.Private != "" && key.Public != "" {
		return key
	}
	newKey, err := newNativeCurveKeyPair()
	if err != nil {
		return key
	}
	return newKey
}

func minDuration(left time.Duration, right time.Duration) time.Duration {
	if right <= 0 || left < right {
		return left
	}
	return right
}

type chatdTransport struct {
	rw            *bufio.ReadWriter
	keys          *chatdTransportKeys
	codec         *binaryNodeCodec
	maxFrameBytes int
}

func (t *chatdTransport) readNode() (chatdNode, error) {
	for {
		ciphertext, err := chatdReadFrame(t.rw.Reader, t.maxFrameBytes)
		if err != nil {
			return chatdNode{}, err
		}
		plaintext, err := t.keys.decrypt(ciphertext)
		if err != nil {
			return chatdNode{}, err
		}
		if len(plaintext) == 0 {
			continue
		}
		return t.codec.decodeNodePayload(plaintext)
	}
}

func (t *chatdTransport) sendNode(node chatdNode) error {
	payload, err := t.codec.encodeNode(node)
	if err != nil {
		return err
	}
	plaintext := append([]byte{0}, payload...)
	ciphertext, err := t.keys.encrypt(plaintext)
	if err != nil {
		return err
	}
	return chatdWriteFrame(t.rw.Writer, ciphertext)
}

func (c *chatdClient) dial(ctx context.Context) (net.Conn, error) {
	address := net.JoinHostPort(c.cfg.Host, strconv.Itoa(c.cfg.Port))
	var conn net.Conn
	var err error
	if strings.TrimSpace(c.cfg.ProxyURL) != "" {
		conn, err = c.dialProxy(ctx, address)
	} else {
		dialer := net.Dialer{Timeout: c.cfg.Timeout}
		conn, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return nil, err
	}
	if !c.cfg.TLS {
		return conn, nil
	}
	tlsConn := tls.Client(conn, &tls.Config{ServerName: c.cfg.Host, InsecureSkipVerify: c.cfg.InsecureTLS})
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return tlsConn, nil
}

func (c *chatdClient) dialProxy(ctx context.Context, target string) (net.Conn, error) {
	parsed, err := url.Parse(c.cfg.ProxyURL)
	if err != nil {
		return nil, err
	}
	switch {
	case strings.HasPrefix(parsed.Scheme, "socks5"):
		return c.dialSOCKS5(ctx, parsed)
	case parsed.Scheme == "http" || parsed.Scheme == "https":
		return c.dialHTTPConnect(ctx, parsed, target)
	default:
		return nil, fmt.Errorf("unsupported chatd proxy scheme %q", parsed.Scheme)
	}
}

func (c *chatdClient) dialHTTPConnect(ctx context.Context, parsed *url.URL, target string) (net.Conn, error) {
	if parsed.Host == "" {
		return nil, fmt.Errorf("proxy host is required")
	}
	dialer := net.Dialer{Timeout: c.cfg.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", parsed.Host)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "https" {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: parsed.Hostname(), InsecureSkipVerify: c.cfg.InsecureTLS})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = conn.Close()
			return nil, err
		}
		conn = tlsConn
	}
	_ = conn.SetDeadline(time.Now().Add(c.cfg.Timeout))
	headers := []string{"CONNECT " + target + " HTTP/1.1", "Host: " + target, "Proxy-Connection: keep-alive", "User-Agent: WhatsApp-CTF-GoChatd/1"}
	if parsed.User != nil {
		password, _ := parsed.User.Password()
		credential := parsed.User.Username() + ":" + password
		headers = append(headers, "Proxy-Authorization: Basic "+base64.StdEncoding.EncodeToString([]byte(credential)))
	}
	if _, err := conn.Write([]byte(strings.Join(headers, "\r\n") + "\r\n\r\n")); err != nil {
		_ = conn.Close()
		return nil, err
	}
	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if !regexp.MustCompile(`^HTTP/\d(?:\.\d)?\s+2\d\d\b`).MatchString(strings.TrimSpace(statusLine)) {
		_ = conn.Close()
		return nil, fmt.Errorf("HTTP CONNECT proxy failed: %s", strings.TrimSpace(statusLine))
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			_ = conn.Close()
			return nil, err
		}
		if line == "\r\n" || line == "\n" {
			break
		}
	}
	return &bufferedConn{Conn: conn, reader: reader}, nil
}

func (c *chatdClient) dialSOCKS5(ctx context.Context, parsed *url.URL) (net.Conn, error) {
	if parsed.Host == "" {
		return nil, fmt.Errorf("SOCKS5 proxy host is required")
	}
	dialer := net.Dialer{Timeout: c.cfg.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", parsed.Host)
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(c.cfg.Timeout))
	methods := []byte{0x00}
	if parsed.User != nil {
		methods = append(methods, 0x02)
	}
	if _, err := conn.Write(append([]byte{0x05, byte(len(methods))}, methods...)); err != nil {
		_ = conn.Close()
		return nil, err
	}
	resp := make([]byte, 2)
	if _, err := io.ReadFull(conn, resp); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if resp[0] != 0x05 || resp[1] == 0xff {
		_ = conn.Close()
		return nil, fmt.Errorf("SOCKS5 proxy rejected authentication methods")
	}
	if resp[1] == 0x02 {
		password, _ := parsed.User.Password()
		username := parsed.User.Username()
		if len(username) > 255 || len(password) > 255 {
			_ = conn.Close()
			return nil, fmt.Errorf("SOCKS5 credentials too long")
		}
		msg := []byte{0x01, byte(len(username))}
		msg = append(msg, username...)
		msg = append(msg, byte(len(password)))
		msg = append(msg, password...)
		if _, err := conn.Write(msg); err != nil {
			_ = conn.Close()
			return nil, err
		}
		if _, err := io.ReadFull(conn, resp); err != nil {
			_ = conn.Close()
			return nil, err
		}
		if resp[1] != 0x00 {
			_ = conn.Close()
			return nil, fmt.Errorf("SOCKS5 username/password authentication failed")
		}
	}
	hostBytes := []byte(c.cfg.Host)
	request := []byte{0x05, 0x01, 0x00, 0x03, byte(len(hostBytes))}
	request = append(request, hostBytes...)
	request = append(request, byte(c.cfg.Port>>8), byte(c.cfg.Port))
	if _, err := conn.Write(request); err != nil {
		_ = conn.Close()
		return nil, err
	}
	head := make([]byte, 4)
	if _, err := io.ReadFull(conn, head); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if head[0] != 0x05 || head[1] != 0x00 {
		_ = conn.Close()
		return nil, fmt.Errorf("SOCKS5 connect failed: %x", head)
	}
	switch head[3] {
	case 0x01:
		_, err = io.CopyN(io.Discard, conn, 4)
	case 0x03:
		var ln [1]byte
		if _, err = io.ReadFull(conn, ln[:]); err == nil {
			_, err = io.CopyN(io.Discard, conn, int64(ln[0]))
		}
	case 0x04:
		_, err = io.CopyN(io.Discard, conn, 16)
	default:
		err = fmt.Errorf("SOCKS5 invalid address type %d", head[3])
	}
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	_, err = io.CopyN(io.Discard, conn, 2)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConn) Read(b []byte) (int, error) { return c.reader.Read(b) }

func decodeRoutingInfo(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if raw, err := base64.RawURLEncoding.DecodeString(value); err == nil {
		return raw, nil
	}
	if raw, err := base64.StdEncoding.DecodeString(value); err == nil {
		return raw, nil
	}
	return []byte(value), nil
}

func resolveLoginIdentity(registeredIdentityID string, state nativeState) (loginIdentity, error) {
	candidates := []string{state.RegistrationJID, registeredIdentityID, state.CC + state.Phone}
	var lastJID string
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		jid := normalizeJID(candidate)
		lastJID = jid
		user := strings.SplitN(strings.SplitN(jid, "@", 2)[0], ":", 2)[0]
		username, err := strconv.ParseUint(user, 10, 64)
		if err == nil {
			return loginIdentity{jid: jid, username: username}, nil
		}
	}
	return loginIdentity{}, fmt.Errorf("cannot derive numeric chatd username from %q", lastJID)
}

func normalizeJID(value string) string {
	value = strings.TrimSpace(value)
	if strings.Contains(value, "@") {
		return value
	}
	compact := regexp.MustCompile(`\D+`).ReplaceAllString(value, "")
	if compact == "" {
		return value
	}
	return compact + "@s.whatsapp.net"
}

func inboundKind(tag string) waappv1.InboundMessageKind {
	switch tag {
	case "message":
		return waappv1.InboundMessageKind_INBOUND_MESSAGE_KIND_MESSAGE
	case "notification":
		return waappv1.InboundMessageKind_INBOUND_MESSAGE_KIND_NOTIFICATION
	case "receipt":
		return waappv1.InboundMessageKind_INBOUND_MESSAGE_KIND_RECEIPT
	default:
		return waappv1.InboundMessageKind_INBOUND_MESSAGE_KIND_SYSTEM
	}
}

func ackStatusForNode(node chatdNode) waappv1.MessageAckStatus {
	if _, ok := buildAckForNode(node); ok {
		return waappv1.MessageAckStatus_MESSAGE_ACK_STATUS_ACKED
	}
	return waappv1.MessageAckStatus_MESSAGE_ACK_STATUS_NOT_REQUIRED
}
