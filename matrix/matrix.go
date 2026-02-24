package matrix

import (
	"context"
	"log"
	"strings"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

func normalizeHomeserverURL(homeserverURL string) string {
	if !strings.Contains(homeserverURL, "://") {
		return "https://" + homeserverURL
	}
	return homeserverURL
}

// NewMatrixWriteCloser logs in to the provided matrix server URL using the provided user ID and password
// and returns a matrix WriteCloser
func NewMatrixWriteCloser(ctx context.Context, userID, userPassword, homeserverURL string) (WriteCloser, error) {
	client, err := mautrix.NewClient(normalizeHomeserverURL(homeserverURL), id.UserID(userID), "")
	if err != nil {
		return nil, err
	}

	log.Print("logging into matrix with username + password")
	_, err = client.Login(ctx, &mautrix.ReqLogin{
		Type: "m.login.password",
		Identifier: mautrix.UserIdentifier{
			Type: "m.id.user",
			User: userID,
		},
		Password:                 userPassword,
		InitialDeviceDisplayName: "",
		StoreCredentials:         true,
	})
	return buildMatrixWriteCloser(client, true), err
}

// NewMatrixWriteCloserWithToken creates a new WriteCloser with the provided user ID and token
func NewMatrixWriteCloserWithToken(userID, token, homeserverURL string) (WriteCloser, error) {
	log.Print("using matrix auth token")
	client, err := mautrix.NewClient(normalizeHomeserverURL(homeserverURL), id.UserID(userID), token)
	if err != nil {
		return nil, err
	}
	return buildMatrixWriteCloser(client, false), err
}

// buildMatrixWriteCloser builds a WriteCloser from a raw matrix client
func buildMatrixWriteCloser(matrixClient *mautrix.Client, closeable bool) WriteCloser {
	return writeCloser{
		writer: writer{
			matrixClient: matrixClient,
		},
		closeable: closeable,
	}
}

type writeCloser struct {
	writer    writer
	closeable bool
}

type writer struct {
	matrixClient *mautrix.Client
}

func (wc writeCloser) GetWriter() Writer {
	return wc.writer
}

func (wc writeCloser) Close(ctx context.Context) error {
	if !wc.closeable {
		return nil
	}
	_, err := wc.writer.matrixClient.Logout(ctx)
	return err
}

func buildFormattedMessagePayload(body FormattedMessage) *event.MessageEventContent {
	return &event.MessageEventContent{
		MsgType:       "m.text",
		Body:          body.TextBody,
		Format:        "org.matrix.custom.html",
		FormattedBody: body.HtmlBody,
	}
}

func (w writer) Send(ctx context.Context, roomID string, body FormattedMessage) (string, error) {
	payload := buildFormattedMessagePayload(body)
	resp, err := w.sendPayload(ctx, roomID, event.EventMessage, payload)
	if err != nil {
		return "", err
	}
	return resp.EventID.String(), err
}

func (w writer) Reply(ctx context.Context, roomID string, eventID string, body FormattedMessage) (string, error) {
	payload := buildFormattedMessagePayload(body)
	payload.RelatesTo = &event.RelatesTo{
		EventID: id.EventID(eventID),
		Type:    event.RelReference,
	}
	resp, err := w.sendPayload(ctx, roomID, event.EventMessage, &payload)
	if err != nil {
		return "", err
	}
	return resp.EventID.String(), err
}

func (w writer) React(ctx context.Context, roomID string, eventID string, reaction string) (string, error) {
	payload := event.ReactionEventContent{
		RelatesTo: event.RelatesTo{
			EventID: id.EventID(eventID),
			Type:    event.RelAnnotation,
			Key:     reaction,
		},
	}
	resp, err := w.sendPayload(ctx, roomID, event.EventReaction, &payload)
	if err != nil {
		return "", err
	}
	return resp.EventID.String(), err
}

func (w writer) sendPayload(ctx context.Context, roomID string, eventType event.Type, messagePayload interface{}) (*mautrix.RespSendEvent, error) {
	return w.matrixClient.SendMessageEvent(ctx, id.RoomID(roomID), eventType, messagePayload)
}
