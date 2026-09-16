package api

import (
	"encoding/json"
	"testing"
)

func TestMergeMaskedSecrets_KeepsStoredSecret(t *testing.T) {
	existing := []byte(`{"host":"h","password":"real-secret"}`)
	incoming := []byte(`{"host":"h","password":"****"}`)

	merged := mergeMaskedSecrets(existing, incoming)
	var m map[string]any
	if err := json.Unmarshal(merged, &m); err != nil {
		t.Fatal(err)
	}
	if m["password"] != "real-secret" {
		t.Fatalf("masked password must keep the stored value, got %v", m["password"])
	}
	if m["host"] != "h" {
		t.Fatalf("non-secret field lost, got %v", m["host"])
	}
}

func TestMergeMaskedSecrets_AllowsRealChange(t *testing.T) {
	existing := []byte(`{"password":"old"}`)
	incoming := []byte(`{"password":"new"}`)

	merged := mergeMaskedSecrets(existing, incoming)
	var m map[string]any
	if err := json.Unmarshal(merged, &m); err != nil {
		t.Fatal(err)
	}
	if m["password"] != "new" {
		t.Fatalf("real password change must be applied, got %v", m["password"])
	}
}

func TestMergeMaskedSecrets_Nested(t *testing.T) {
	existing := []byte(`{"opensearch":{"password":"real","index":"old"}}`)
	incoming := []byte(`{"opensearch":{"password":"****","index":"new"}}`)

	merged := mergeMaskedSecrets(existing, incoming)
	var m map[string]any
	if err := json.Unmarshal(merged, &m); err != nil {
		t.Fatal(err)
	}
	os := m["opensearch"].(map[string]any)
	if os["password"] != "real" {
		t.Fatalf("nested masked password must keep the stored value, got %v", os["password"])
	}
	if os["index"] != "new" {
		t.Fatalf("nested real change lost, got %v", os["index"])
	}
}

func TestMergeMaskedSecrets_EmptyIncomingKeepsExisting(t *testing.T) {
	existing := []byte(`{"password":"real"}`)
	if got := mergeMaskedSecrets(existing, nil); string(got) != string(existing) {
		t.Fatalf("empty incoming must return existing, got %s", got)
	}
}

func TestMergeMaskedSecrets_InvalidJSON(t *testing.T) {
	existing := []byte(`{"password":"real"}`)
	incoming := []byte(`not-json`)
	if got := mergeMaskedSecrets(existing, incoming); string(got) != string(incoming) {
		t.Fatalf("invalid incoming must be returned untouched, got %s", got)
	}
}
