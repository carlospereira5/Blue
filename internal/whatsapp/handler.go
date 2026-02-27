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

	if time.Since(msg.Info.Timestamp) > 30*time.Second {
		log.Printf("[whatsapp] mensaje offline ignorado (%s)", msg.Info.Timestamp.Format(time.RFC3339))
		return
	}

	if b.groupJID.IsEmpty() {
		if msg.Info.IsGroup {
			log.Printf("[whatsapp] [discovery] grupo detectado: %s", msg.Info.Chat)
			return
		}
	} else {
		if !msg.Info.IsGroup || msg.Info.Chat != b.groupJID {
			return
		}
	}

	if msg.Info.IsFromMe {
		return
	}

	if !b.isAllowed(msg.Info.Sender) {
		log.Printf("[whatsapp] mensaje rechazado de %s", msg.Info.Sender)
		b.sendReply(context.Background(), msg.Info.Chat, "No estás autorizado para usar Lumi.")
		return
	}

	// Goroutine independiente para manejar I/O de red sin bloquear whatsmeow
	go func() {
		ctx := context.Background()
		var text string

		// Verificamos si es una nota de voz/audio
		if audioMsg := msg.Message.GetAudioMessage(); audioMsg != nil {
			log.Printf("[whatsapp] descargando nota de voz de %s...", msg.Info.Sender)
			
			// FIX: Pasamos 'ctx' como primer argumento para satisfacer la firma actual de whatsmeow
			audioData, err := b.client.Download(ctx, audioMsg)
			if err != nil {
				log.Printf("[whatsapp] error descargando audio: %v", err)
				return
			}
			
			// Enviamos los bytes crudos (OGG) a Groq
			text, err = b.agent.TranscribeAudio(ctx, audioData)
			if err != nil {
				log.Printf("[whatsapp] error transcribiendo audio: %v", err)
				b.sendReply(ctx, msg.Info.Chat, "Lo siento, tuve un problema al procesar tu nota de voz 🎙️❌")
				return
			}
			
			log.Printf("[whatsapp] audio transcrito (%s): %q", msg.Info.Sender, text)
		} else {
			// Es un mensaje de texto normal
			text = extractText(msg)
		}

		// Si el texto final está vacío (ya sea porque no había texto o el audio estaba en silencio) salimos
		if text == "" {
			return
		}

		b.processMessage(msg.Info.Chat, msg.Info.Sender.String(), text)
	}()
}

func (b *Bot) processMessage(chat types.JID, senderID string, text string) {
	ctx := context.Background()
	response, err := b.agent.Chat(ctx, senderID, text)
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
		return true
	}

	clean := types.NewJID(sender.User, sender.Server)
	if b.allowed[clean] {
		return true
	}

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
