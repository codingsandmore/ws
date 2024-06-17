package ws

import (
	"errors"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	"net/http"
	"net/url"
	"time"
)

type MessageDecoder interface {
	Decode(bytes []byte, client WebSocketClient) (any, error)
}
type WebSocketClient interface {
	Connect() error
	Send(any)
	Subscribe(messasge chan any, decoder MessageDecoder, welcomeMessage any) error
	Shutdown()
}
type DefaultWebSocketClient struct {
	Header           http.Header
	Addr             string
	Conn             *websocket.Conn
	IncomingMessages chan any
	OutgoingMessages chan any
	Decoder          MessageDecoder
	welcomeMessage   any
}

func NewDefaultClient(addr string, header http.Header) *DefaultWebSocketClient {

	return &DefaultWebSocketClient{
		Header:           header,
		Addr:             addr,
		Conn:             nil,
		IncomingMessages: make(chan any),
		OutgoingMessages: make(chan any, 10), // Change 10 to appropriate buffer size
	}
}
func (c *DefaultWebSocketClient) Shutdown() {
	defer c.Conn.Close()
	defer close(c.OutgoingMessages)
	defer close(c.IncomingMessages)

}

func (c *DefaultWebSocketClient) Subscribe(messages chan any, decoder MessageDecoder, welcomeMessage any) error {
	if decoder == nil {
		log.Error().Msgf("you need to provide a decoder or this process will fail!")
		return errors.New("please provide a decoder")
	} else {
		log.Info().Any("decoder", decoder).Msgf("subscribing to messages")
	}
	c.welcomeMessage = welcomeMessage
	c.Decoder = decoder
	c.Start()

	for {
		select {
		case m := <-c.IncomingMessages:
			if m != nil {
				messages <- m
			} else {
				log.Warn().Msgf("ignoring Nil message!")
			}

		}
	}
}
func (c *DefaultWebSocketClient) Connect() error {
	log.Debug().Str("url", c.Addr).Msgf("connecting to %v", c.Addr)
	u, err := url.Parse(c.Addr)
	if err != nil {
		log.Error().AnErr("error during parsing of url", err)
		return err
	}

	c.Conn, _, err = websocket.DefaultDialer.Dial(u.String(), c.Header)
	if err != nil {
		log.Error().AnErr("error during dialing url", err)
		return err
	}

	if c.welcomeMessage != nil {
		log.Debug().Any("welcome", c.welcomeMessage).Msgf("sending welcome message")
		c.Send(c.welcomeMessage)
	}

	return nil
}

func (c *DefaultWebSocketClient) maintainConnection() {
	for {
		if c.Conn == nil {
			log.Info().Msgf("attempting to establish connection to %v", c.Addr)
			if err := c.Connect(); err != nil {
				log.Error().AnErr("connect error", err).Msgf("Connect Error:", err)
				time.Sleep(time.Second)
				continue
			}
		}

		_, msg, err := c.Conn.ReadMessage()
		if err != nil {
			log.Error().AnErr("read error", err).Msgf("Read Error:", err)
			c.Conn = nil
			continue
		}

		message, err := c.Decoder.Decode(msg, c)

		if err != nil {
			log.Error().AnErr("decode error", err).Msgf("Decode Error: %s", err)
		} else if message == nil {
			log.Debug().Msgf("ignored nil message")
		} else {
			log.Debug().Any("message", message).Msgf("forwarding decoded message")
			c.IncomingMessages <- message
		}
	}
}

func (c *DefaultWebSocketClient) maintainOutgoingMessages() {
	for msg := range c.OutgoingMessages {
		if c.Conn == nil {
			log.Error().Msgf("Connection Error: No active connection, Sending message back to queue")
			time.Sleep(500 * time.Millisecond)
			c.OutgoingMessages <- msg
			continue
		}

		var err error
		switch v := msg.(type) {
		case []byte:
			log.Debug().Str("send", string(v)).Msgf("sending message")
			err = c.Conn.WriteMessage(1, v)
		case string:
			err = c.Conn.WriteMessage(1, []byte(v))

		default:
			err = c.Conn.WriteJSON(v)
		}

		if err != nil {
			log.Error().AnErr("write error", err).Msgf("Write Error:", err)
			c.Conn = nil
			continue
		}
	}
	log.Info().Msgf("finished sending loop")
}

func (c *DefaultWebSocketClient) Send(msg any) {
	log.Debug().Any("message", msg).Msg("Sending message")
	c.OutgoingMessages <- msg
}

func (c *DefaultWebSocketClient) Start() {
	go c.maintainConnection()
	go c.maintainOutgoingMessages()
}
