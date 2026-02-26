package whatsapp

import (
	"context"
	"log"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func (b *Bot) handleEvent(evt interface{}) {
	msg, ok := evt.(*events.Message)
	if !ok {
		return
	}

	// Ignorar mensajes offline (buffered durante desconexión).
	if time.Since(msg.Info.Timestamp) > 30*time.Second {
		log.Printf("[whatsapp] mensaje offline ignorado (%s)", msg.Info.Timestamp.Format(time.RFC3339))
		return
	}

	// Solo DMs, ignorar grupos.
	if msg.Info.IsGroup {
		return
	}

	text := extractText(msg)
	if text == "" {
		return
	}

	if !b.isAllowed(msg.Info.Sender) {
		log.Printf("[whatsapp] mensaje rechazado de %s", msg.Info.Sender)
		b.sendReply(context.Background(), msg.Info.Chat, "No estás autorizado para usar Lumi.")
		return
	}

	log.Printf("[whatsapp] mensaje de %s: %s", msg.Info.Sender, text)

	// Goroutine para no bloquear el event loop — agent.Chat() tarda 3-5s.
	go b.processMessage(msg.Info.Chat, text)
}

func (b *Bot) processMessage(chat types.JID, text string) {
	ctx := context.Background()
	response, err := b.agent.Chat(ctx, text)
	if err != nil {
		log.Printf("[whatsapp] error en Chat: %v", err)
		b.sendReply(ctx, chat, "Error procesando tu consulta. Intenta de nuevo.")
		return
	}
	b.sendReply(ctx, chat, response)
}

func (b *Bot) sendReply(ctx context.Context, chat types.JID, text string) {
	_, err := b.client.SendMessage(ctx, chat, &waE2E.Message{
		Conversation: proto.String(text),
	})
	if err != nil {
		log.Printf("[whatsapp] error enviando mensaje: %v", err)
	}
}

func extractText(msg *events.Message) string {
	if c := msg.Message.GetConversation(); c != "" {
		return c
	}
	if ext := msg.Message.GetExtendedTextMessage(); ext != nil {
		return ext.GetText()
	}
	return ""
}

func (b *Bot) isAllowed(sender types.JID) bool {
	if len(b.allowed) == 0 {
		return true // Sin whitelist = todos permitidos.
	}

	// Comparar sin device ID para que matchee cualquier dispositivo del mismo número.
	clean := types.NewJID(sender.User, sender.Server)
	if b.allowed[clean] {
		return true
	}

	// WhatsApp puede mandar el sender como LID (@lid) en vez de número (@s.whatsapp.net).
	// Resolvemos LID → número de teléfono usando el store de whatsmeow.
	if sender.Server == types.HiddenUserServer {
		pn, err := b.client.Store.GetAltJID(context.Background(), sender)
		if err == nil && !pn.IsEmpty() {
			resolved := types.NewJID(pn.User, pn.Server)
			if b.allowed[resolved] {
				return true
			}
		}
	}

	return false
}
