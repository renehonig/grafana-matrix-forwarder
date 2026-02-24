package service

import (
	"context"
	"grafana-matrix-forwarder/cfg"
	"grafana-matrix-forwarder/formatter"
	"grafana-matrix-forwarder/matrix"
	"grafana-matrix-forwarder/model"
	"log"
)

type Forwarder struct {
	AppSettings                cfg.AppSettings
	MatrixWriter               matrix.Writer
	alertToSentEventMap        map[string]sentMatrixEvent
	alertMapPersistenceEnabled bool
}

func NewForwarder(appSettings cfg.AppSettings, writer matrix.Writer) Forwarder {
	return Forwarder{
		AppSettings:                appSettings,
		MatrixWriter:               writer,
		alertToSentEventMap:        map[string]sentMatrixEvent{},
		alertMapPersistenceEnabled: appSettings.PersistAlertMap,
	}
}

func (f *Forwarder) ForwardEvents(ctx context.Context, roomIds []string, alerts []model.AlertData) error {
	for _, id := range roomIds {
		for _, alert := range alerts {
			err := f.forwardSingleEvent(ctx, id, alert)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *Forwarder) forwardSingleEvent(ctx context.Context, roomID string, alert model.AlertData) error {
	log.Printf("alert received (%s) - forwarding to room: %v", alert.Id, roomID)

	resolveWithReaction := f.AppSettings.ResolveMode == cfg.ResolveWithReaction
	resolveWithReply := f.AppSettings.ResolveMode == cfg.ResolveWithReply

	if sentEvent, ok := f.alertToSentEventMap[alert.Id]; ok {
		if alert.State == model.AlertStateResolved && resolveWithReaction {
			return f.sendResolvedReaction(ctx, roomID, sentEvent.EventID, alert)
		}
		if alert.State == model.AlertStateResolved && resolveWithReply {
			return f.sendResolvedReply(ctx, roomID, sentEvent, alert)
		}
	}
	return f.sendAlertMessage(ctx, roomID, alert)
}

func (f *Forwarder) sendResolvedReaction(ctx context.Context, roomID, eventID string, alert model.AlertData) error {
	reaction := formatter.GenerateReaction(alert)
	f.deleteMatrixEvent(alert.Id)
	_, err := f.MatrixWriter.React(ctx, roomID, eventID, reaction)
	return err
}

func (f *Forwarder) sendResolvedReply(ctx context.Context, roomID string, sentEvent sentMatrixEvent, alert model.AlertData) error {
	reply, err := formatter.GenerateReply(sentEvent.SentFormattedBody, alert)
	if err != nil {
		return err
	}
	f.deleteMatrixEvent(alert.Id)
	_, err = f.MatrixWriter.Reply(ctx, roomID, sentEvent.EventID, reply)
	return err
}

func (f *Forwarder) sendAlertMessage(ctx context.Context, roomID string, alert model.AlertData) error {
	message, err := formatter.GenerateMessage(alert, f.AppSettings.MetricRounding)
	if err != nil {
		return err
	}
	resp, err := f.MatrixWriter.Send(ctx, roomID, message)
	if err == nil {
		f.storeMatrixEvent(alert.Id, resp, message.HtmlBody)
	}
	return err
}
