package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/gorilla/websocket"
)

// ControlClose represents a control close message to indicate an error
// and to ultimately close the underlying websocket connection.
// It reimplements the actual control close message available in the websocket
// protocol to overcome the limitation of 125 bytes.
// See https://www.rfc-editor.org/rfc/rfc6455#section-5.5.
type ControlClose struct {
	ControlMessage string `json:"control_message"`
}

// WebsocketGateway represents a utility wrapper for websocket connections.
type WebsocketGateway struct {
	reader chan []byte
	ctx    context.Context

	conn *websocket.Conn
	// There can only be one writer on the connection at a time.
	// In case the outer context gets cancelled there can be a situation
	// in which writing the contexts error cause can collide
	// with a normal write to the websocket.
	writeLock sync.Mutex
}

// NewWebsocketGateway returns a new websocket wrapper allowing to easily write and consume
// messages to/from the underlying websocket connection.
// It allows providing a context which is cancelled as soon as the underlying websocket connection
// is closed by either side of the connection.
func NewWebsocketGateway(ctx context.Context, conn *websocket.Conn) *WebsocketGateway {
	gw := &WebsocketGateway{
		reader: make(chan []byte),
		conn:   conn,
	}

	gwCtx, gwCancel := context.WithCancelCause(ctx)
	gw.ctx = gwCtx

	go func() {
		<-gwCtx.Done()

		// Send close control message.
		// Try to send the cause from the outer context if present.
		_ = gw.WriteClose(context.Cause(gwCtx))

		// Shutdown the read loop.
		_ = gw.conn.Close()
	}()

	go func() {
		defer close(gw.reader)

		for {
			_, reader, err := conn.ReadMessage()
			if err != nil {
				// If the connection got closed due to the outer context, return this error instead.
				if ctx.Err() != nil {
					// Try to use the cause from the outer context if present.
					err = context.Cause(ctx)
				}

				// Cancel the inner context too with the respective error.
				defer gwCancel(err)
				return
			}

			// Cancel in case we have received our own control close message.
			// To not get confused with other JSON payloads, we identify our
			// control close message by requiring the "control_message" field.
			controlClose := ControlClose{}
			decoder := json.NewDecoder(bytes.NewReader(reader))
			decoder.DisallowUnknownFields()
			err = decoder.Decode(&controlClose)
			if err == nil {
				// Cancel the inner context with the respective error.
				defer gwCancel(errors.New(controlClose.ControlMessage))
				return
			}

			gw.reader <- reader
		}
	}()

	return gw
}

// Receive returns the inner channel which allows reading from the websocket connection.
// If used together with other channels ensure to also consume the gateway's context
// in order to get informed about a potentially closed connection.
// If there aren't other channels that need to be consumed in parallel use ReceiveCombined instead.
func (w *WebsocketGateway) Receive() <-chan []byte {
	return w.reader
}

// ReceiveWithContext tries to read from the websocket connection and unmarshals
// the received data into v.
// It's waiting on both the websocket connection and the either of the contexts and returns
// whatever is returning/cancelled first.
func (w *WebsocketGateway) ReceiveWithContext(ctx context.Context, v any) error {
	var err error
	select {
	case bytes := <-w.Receive():
		err = json.Unmarshal(bytes, v)
	case <-w.ctx.Done():
		err = context.Cause(w.ctx)
	case <-ctx.Done():
		err = context.Cause(ctx)
	}

	return err
}

// Context returns the inner gateway's context.
// It's getting cancelled if the outer context is cancelled or the websocket connection is closed.
func (w *WebsocketGateway) Context() context.Context {
	return w.ctx
}

// Write writes the given data onto the websocket connection.
func (w *WebsocketGateway) Write(v any) error {
	w.writeLock.Lock()
	defer w.writeLock.Unlock()

	if w.ctx.Err() != nil {
		return context.Cause(w.ctx)
	}

	return w.conn.WriteJSON(v)
}

// WriteClose sends our websocket control close message.
// Unlike the actual websocket control close message this supports message longer than 125 bytes
// as well as special characters.
// It waits for the other side to hang up or the gateway's context being cancelled.
func (w *WebsocketGateway) WriteClose(err error) error {
	writeErr := w.Write(ControlClose{
		ControlMessage: err.Error(),
	})
	if writeErr != nil {
		return fmt.Errorf("Failed to write control message: %w", writeErr)
	}

	// Wait on the other end to hang up.
	// Our inner context gets cancelled if the websocket connection is closed.
	<-w.Context().Done()

	return nil
}
