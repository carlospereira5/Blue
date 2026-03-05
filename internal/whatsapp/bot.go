// Package whatsapp integra whatsmeow para recibir y enviar mensajes de WhatsApp.
package whatsapp

import (
	"context"
	"fmt"
	"log"
	"os"

	_ "modernc.org/sqlite"
	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"

	"aria/internal/agent"
)

// Bot es el wrapper de whatsmeow que conecta WhatsApp con el Agent de Lumi.
type Bot struct {
	client   *whatsmeow.Client
	agent    *agent.Agent
	allowed  map[types.JID]bool
	groupJID types.JID
}

// New crea un Bot de WhatsApp listo para conectar.
// allowedNumbers son los números autorizados en formato "5491112345678".
// groupJID es el JID del grupo donde Lumi escucha (vacío = discovery mode, DMs).
func New(ctx context.Context, ag *agent.Agent, dbPath string, allowedNumbers []string, groupJID string) (*Bot, error) {
	dbLog := waLog.Stdout("DB", "WARN", true)
	dsn := "file:" + dbPath + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	container, err := sqlstore.New(ctx, "sqlite", dsn, dbLog)
	if err != nil {
		return nil, fmt.Errorf("whatsapp sqlstore: %w", err)
	}

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, fmt.Errorf("whatsapp device store: %w", err)
	}

	clientLog := waLog.Stdout("WA", "WARN", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)

	allowed := make(map[types.JID]bool, len(allowedNumbers))
	for _, num := range allowedNumbers {
		jid := types.NewJID(num, types.DefaultUserServer)
		allowed[jid] = true
	}

	bot := &Bot{
		client:  client,
		agent:   ag,
		allowed: allowed,
	}

	if groupJID != "" {
		parsed, err := types.ParseJID(groupJID)
		if err != nil {
			return nil, fmt.Errorf("parsing WHATSAPP_GROUP_JID %q: %w", groupJID, err)
		}
		if parsed.Server != types.GroupServer {
			return nil, fmt.Errorf("WHATSAPP_GROUP_JID %q no es un grupo (esperado @g.us)", groupJID)
		}
		bot.groupJID = parsed
		log.Printf("[whatsapp] modo grupo: solo escucha en %s", parsed)
	} else {
		log.Println("[whatsapp] modo discovery: escucha DMs, logea grupos detectados")
	}

	return bot, nil
}

// Start conecta al servidor de WhatsApp y bloquea hasta que ctx se cancele.
// Si no hay sesión guardada, muestra un QR para escanear.
func (b *Bot) Start(ctx context.Context) error {
	b.client.AddEventHandler(b.handleEvent)

	if b.client.Store.ID == nil {
		if err := b.loginWithQR(ctx); err != nil {
			return fmt.Errorf("whatsapp QR login: %w", err)
		}
	} else {
		if err := b.client.Connect(); err != nil {
			return fmt.Errorf("whatsapp connect: %w", err)
		}
		log.Println("[whatsapp] conectado con sesión existente")
	}

	<-ctx.Done()
	log.Println("[whatsapp] shutting down...")
	b.client.Disconnect()
	return nil
}

func (b *Bot) loginWithQR(ctx context.Context) error {
	qrChan, _ := b.client.GetQRChannel(ctx)
	if err := b.client.Connect(); err != nil {
		return err
	}

	for evt := range qrChan {
		switch evt.Event {
		case "code":
			fmt.Println()
			fmt.Println("  Escaneá este QR con WhatsApp:")
			fmt.Println()
			qrCfg := qrterminal.Config{
				Level:     qrterminal.L,
				Writer:    os.Stdout,
				HalfBlocks: true,
				BlackChar: qrterminal.BLACK_BLACK,
				WhiteChar: qrterminal.WHITE_WHITE,
				QuietZone: 1,
			}
			qrterminal.GenerateWithConfig(evt.Code, qrCfg)
			fmt.Println()
		case "success":
			log.Println("[whatsapp] login exitoso")
			return nil
		case "timeout":
			return fmt.Errorf("QR timeout — reiniciá el bot")
		}
	}
	return nil
}
