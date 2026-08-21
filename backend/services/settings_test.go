package services

import "testing"

func TestSettingsDefaultsAndPersist(t *testing.T) {
	dir := t.TempDir()

	// Fresh install → defaults (all features on).
	s := NewSettingsStore(dir)
	got := s.Get()
	if got.InstanceName != "CloudDrive" || !got.OffsiteBackupEnabled || !got.SharingEnabled {
		t.Fatalf("unexpected defaults: %+v", got)
	}

	// Update + persist (turn offsite off).
	if err := s.Update(Settings{InstanceName: "Home Cloud", OffsiteBackupEnabled: false, SharingEnabled: true}); err != nil {
		t.Fatal(err)
	}

	// A brand-new store reads the persisted values over the defaults — the
	// `false` must survive (not revert to the default `true`).
	s2 := NewSettingsStore(dir)
	got2 := s2.Get()
	if got2.InstanceName != "Home Cloud" || got2.OffsiteBackupEnabled || !got2.SharingEnabled {
		t.Errorf("persisted settings wrong: %+v", got2)
	}

	// An empty instance name falls back to the default rather than a blank title.
	if err := s2.Update(Settings{InstanceName: "", OffsiteBackupEnabled: true, SharingEnabled: false}); err != nil {
		t.Fatal(err)
	}
	if s2.Get().InstanceName != "CloudDrive" {
		t.Errorf("empty name should fall back to default, got %q", s2.Get().InstanceName)
	}
}
