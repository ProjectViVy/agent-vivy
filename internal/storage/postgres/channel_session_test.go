package postgres

import (
	"errors"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/migrations"
)

func TestChannelSessionIdentityAndPreflightPostgres(t *testing.T) {
	ctx, db, manifest := generationFixturePool(t)
	buildGenerationPrefix(t, ctx, db, manifest, len(manifest.Migrations(migrations.Postgres)))
	b := &Backend{db: &DB{SQL: db}}
	binding := &domain.ChannelSessionBinding{Channel: "fake", ChatID: "chat/raw", TopicID: ""}
	if err := b.CreateSession(ctx, domain.Session{ID: "old-opaque", CreatedAt: 1, ChannelBinding: binding}); err != nil {
		t.Fatal(err)
	}
	if id, err := b.FindChannelSession(ctx, *binding); err != nil || id != "old-opaque" {
		t.Fatalf("lookup=%q %v", id, err)
	}
	got, err := b.GetSession(ctx, "old-opaque")
	if err != nil || got.ChannelBinding == nil || *got.ChannelBinding != *binding {
		t.Fatalf("binding roundtrip=%+v %v", got, err)
	}
	if err := b.CreateSession(ctx, domain.Session{ID: "different-id", CreatedAt: 2, ChannelBinding: binding}); err == nil {
		t.Fatal("duplicate tuple admitted")
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO snapshots(key,value,version) VALUES('observer/run/retired-key',decode('31','hex'),7)`); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Apply(ctx, db, migrations.Postgres); !errors.Is(err, migrations.ErrUnsupportedSourceFormat) {
		t.Fatalf("unconverted observer state admitted: %v", err)
	}
	var value []byte
	var version int
	if err := db.QueryRowContext(ctx, `SELECT value,version FROM snapshots`).Scan(&value, &version); err != nil || string(value) != "1" || version != 7 {
		t.Fatalf("progress changed: %q %d %v", value, version, err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM snapshots; INSERT INTO sessions(id,title,created_at) VALUES('sess_ch_oldopaque','history',1)`); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Apply(ctx, db, migrations.Postgres); !errors.Is(err, migrations.ErrUnsupportedSourceFormat) {
		t.Fatalf("unbound channel state admitted: %v", err)
	}
}
