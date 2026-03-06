package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ── user_profiles ─────────────────────────────────────────────────────────────

// GetUserProfile retorna el perfil del usuario con el JID dado.
// Retorna (profile, true, nil) si existe, (zero, false, nil) si no está configurado.
func (s *SQLStore) GetUserProfile(ctx context.Context, jid string) (UserProfile, bool, error) {
	q := fmt.Sprintf(
		`SELECT jid, COALESCE(name,''), COALESCE(role,''), COALESCE(notes,'')
		FROM user_profiles WHERE jid = %s`,
		s.dialect.Placeholder(1),
	)
	var p UserProfile
	err := s.db.QueryRowContext(ctx, q, jid).Scan(&p.JID, &p.Name, &p.Role, &p.Notes)
	if err == sql.ErrNoRows {
		return UserProfile{}, false, nil
	}
	if err != nil {
		return UserProfile{}, false, fmt.Errorf("db: get user profile %q: %w", jid, err)
	}
	return p, true, nil
}

// UpsertUserProfile crea o actualiza el perfil del usuario.
// Si el JID ya existe, actualiza name, role, notes y updated_at.
func (s *SQLStore) UpsertUserProfile(ctx context.Context, p UserProfile) error {
	now := formatTime(time.Now())
	q := fmt.Sprintf(
		`INSERT INTO user_profiles (jid, name, role, notes, created_at, updated_at)
		VALUES (%s)
		ON CONFLICT(jid) DO UPDATE SET
			name=EXCLUDED.name,
			role=EXCLUDED.role,
			notes=EXCLUDED.notes,
			updated_at=EXCLUDED.updated_at`,
		s.dialect.Placeholders(1, 6),
	)
	_, err := s.db.ExecContext(ctx, q, p.JID, p.Name, p.Role, p.Notes, now, now)
	if err != nil {
		return fmt.Errorf("db: upsert user profile %q: %w", p.JID, err)
	}
	return nil
}

// ── user_memories ─────────────────────────────────────────────────────────────

// GetUserMemories retorna las últimas 10 memorias del usuario, de más reciente a más antigua.
func (s *SQLStore) GetUserMemories(ctx context.Context, jid string) ([]UserMemory, error) {
	q := fmt.Sprintf(
		`SELECT id, jid, content, created_at FROM user_memories
		WHERE jid = %s ORDER BY id DESC LIMIT 10`,
		s.dialect.Placeholder(1),
	)
	rows, err := s.db.QueryContext(ctx, q, jid)
	if err != nil {
		return nil, fmt.Errorf("db: get user memories %q: %w", jid, err)
	}
	defer rows.Close()

	var memories []UserMemory
	for rows.Next() {
		var m UserMemory
		var createdAtStr string
		if err := rows.Scan(&m.ID, &m.JID, &m.Content, &createdAtStr); err != nil {
			return nil, fmt.Errorf("db: scan user memory: %w", err)
		}
		m.CreatedAt = parseTime(createdAtStr)
		memories = append(memories, m)
	}
	return memories, rows.Err()
}

// SaveUserMemory guarda una nueva memoria sobre el usuario.
func (s *SQLStore) SaveUserMemory(ctx context.Context, jid, content string) error {
	q := fmt.Sprintf(
		`INSERT INTO user_memories (jid, content, created_at) VALUES (%s)`,
		s.dialect.Placeholders(1, 3),
	)
	_, err := s.db.ExecContext(ctx, q, jid, content, formatTime(time.Now()))
	if err != nil {
		return fmt.Errorf("db: save user memory: %w", err)
	}
	return nil
}

// DeleteUserMemory elimina una memoria por ID. Útil para correcciones manuales.
func (s *SQLStore) DeleteUserMemory(ctx context.Context, id int64) error {
	q := fmt.Sprintf(
		`DELETE FROM user_memories WHERE id = %s`,
		s.dialect.Placeholder(1),
	)
	_, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("db: delete user memory %d: %w", id, err)
	}
	return nil
}
