package app

import (
	"context"
	"testing"

	"github.com/Yacobolo/leapview/internal/dashboard/publication"
	publicationsqlite "github.com/Yacobolo/leapview/internal/dashboard/publication/sqlite"
)

func TestPublicationStreamRegistryClosesStaleGenerationAndPublicID(t *testing.T) {
	registry := publication.NewMemoryStreamRegistry()
	ctx, unregister, err := registry.Register(context.Background(), "publication", "stream", publication.StreamVersion{
		PublicID: "public-old", ServingStateID: "state-old",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer unregister()

	registry.Reconcile(context.Background(), map[string]publication.StreamVersion{
		"publication": {PublicID: "public-new", ServingStateID: "state-new"},
	})
	select {
	case <-ctx.Done():
	default:
		t.Fatal("stale publication stream remained active")
	}
}

func TestPublicationStreamRegistryKeepsCurrentGeneration(t *testing.T) {
	registry := publication.NewMemoryStreamRegistry()
	version := publication.StreamVersion{PublicID: "public", ServingStateID: "state"}
	ctx, unregister, err := registry.Register(context.Background(), "publication", "stream", version)
	if err != nil {
		t.Fatal(err)
	}
	defer unregister()

	registry.Reconcile(context.Background(), map[string]publication.StreamVersion{"publication": version})
	select {
	case <-ctx.Done():
		t.Fatal("current publication stream was closed")
	default:
	}
}

func TestDurablePublicationStreamRegistryClosesSupersededLocalRegistration(t *testing.T) {
	store := testStore(t)
	seedActivePublication(t, store, "public")
	first := publicationsqlite.NewStreamRegistry(store.SQLDB())
	second := publicationsqlite.NewStreamRegistry(store.SQLDB())
	version := publication.StreamVersion{PublicID: "public", ServingStateID: "state"}

	firstContext, unregisterFirst, err := first.Register(context.Background(), "pub_website", "stream", version)
	if err != nil {
		t.Fatal(err)
	}
	defer unregisterFirst()
	_, unregisterSecond, err := second.Register(context.Background(), "pub_website", "stream", version)
	if err != nil {
		t.Fatal(err)
	}
	defer unregisterSecond()

	first.Reconcile(context.Background(), map[string]publication.StreamVersion{"pub_website": version})
	select {
	case <-firstContext.Done():
	default:
		t.Fatal("superseded local publication stream remained active")
	}
}
