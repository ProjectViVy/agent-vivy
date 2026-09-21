package migrations

import "testing"

func TestPostgresLegacyStateUsesVerifiedSchemaShape(t *testing.T) {
	tests := []struct {
		name  string
		max   int64
		shape postgresLegacyShape
		want  int64
	}{
		{
			name: "v14 compaction baseline",
			max:  14,
			shape: postgresLegacyShape{
				sessionCompactions: true,
			},
			want: 15,
		},
		{
			name: "accumulated v15 bootstrap",
			max:  15,
			shape: postgresLegacyShape{
				sessionCompactions:  true,
				channelProvenance:   true,
				cronJobs:            true,
				messageAttachments:  true,
				fileVersions:        true,
				fileReads:           true,
				sessionTruncations:  true,
				messageFileContexts: true,
				sessionUpdatedAt:    true,
				workspacePath:       true,
			},
			want: 23,
		},
		{
			name: "accumulated v16 bootstrap",
			max:  16,
			shape: postgresLegacyShape{
				sessionCompactions:  true,
				channelProvenance:   true,
				cronJobs:            true,
				messageAttachments:  true,
				fileVersions:        true,
				fileReads:           true,
				sessionTruncations:  true,
				messageFileContexts: true,
				sessionUpdatedAt:    true,
				workspacePath:       true,
			},
			want: 23,
		},
		{
			name: "accumulated v17 bootstrap",
			max:  17,
			shape: postgresLegacyShape{
				sessionCompactions:  true,
				channelProvenance:   true,
				cronJobs:            true,
				messageAttachments:  true,
				fileVersions:        true,
				fileReads:           true,
				sessionTruncations:  true,
				messageFileContexts: true,
				sessionUpdatedAt:    true,
				workspacePath:       true,
			},
			want: 23,
		},
		{
			name: "accumulated v18 bootstrap",
			max:  18,
			shape: postgresLegacyShape{
				sessionCompactions:  true,
				channelProvenance:   true,
				cronJobs:            true,
				messageAttachments:  true,
				fileVersions:        true,
				fileReads:           true,
				sessionTruncations:  true,
				messageFileContexts: true,
				sessionUpdatedAt:    true,
				workspacePath:       true,
			},
			want: 23,
		},
		{
			name: "v19 upgrade chain",
			max:  19,
			shape: postgresLegacyShape{
				sessionCompactions:  true,
				channelProvenance:   true,
				cronJobs:            true,
				messageAttachments:  true,
				fileVersions:        true,
				fileReads:           true,
				sessionTruncations:  true,
				messageFileContexts: true,
			},
			want: 21,
		},
		{
			name: "v19 historical chain missing truncations",
			max:  19,
			shape: postgresLegacyShape{
				sessionCompactions:  true,
				channelProvenance:   true,
				cronJobs:            true,
				messageAttachments:  true,
				fileVersions:        true,
				fileReads:           true,
				messageFileContexts: true,
			},
			want: 19,
		},
		{
			name: "v20 upgrade chain",
			max:  20,
			shape: postgresLegacyShape{
				sessionCompactions:  true,
				channelProvenance:   true,
				cronJobs:            true,
				messageAttachments:  true,
				fileVersions:        true,
				fileReads:           true,
				sessionTruncations:  true,
				messageFileContexts: true,
				sessionUpdatedAt:    true,
			},
			want: 22,
		},
		{
			name: "v20 historical chain missing truncations",
			max:  20,
			shape: postgresLegacyShape{
				sessionCompactions:  true,
				channelProvenance:   true,
				cronJobs:            true,
				messageAttachments:  true,
				fileVersions:        true,
				fileReads:           true,
				messageFileContexts: true,
				sessionUpdatedAt:    true,
			},
			want: 19,
		},
		{
			name: "v21 upgrade chain",
			max:  21,
			shape: postgresLegacyShape{
				sessionCompactions:  true,
				channelProvenance:   true,
				cronJobs:            true,
				messageAttachments:  true,
				fileVersions:        true,
				fileReads:           true,
				sessionTruncations:  true,
				messageFileContexts: true,
				sessionUpdatedAt:    true,
				workspacePath:       true,
			},
			want: 23,
		},
		{
			name: "v21 historical chain missing truncations",
			max:  21,
			shape: postgresLegacyShape{
				sessionCompactions:  true,
				channelProvenance:   true,
				cronJobs:            true,
				messageAttachments:  true,
				fileVersions:        true,
				fileReads:           true,
				messageFileContexts: true,
				sessionUpdatedAt:    true,
				workspacePath:       true,
			},
			want: 19,
		},
		{
			name: "v13 baseline",
			max:  13,
			want: 13,
		},
		{
			name: "cron-only reconciliation",
			max:  15,
			shape: postgresLegacyShape{
				sessionCompactions: true,
				cronJobs:           true,
			},
			want: 15,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := legacyPostgresState(test.max, test.shape)
			if err != nil {
				t.Fatalf("legacyPostgresState: %v", err)
			}
			if got != test.want {
				t.Fatalf("legacyPostgresState = %d, want %d", got, test.want)
			}
		})
	}
}

func TestPostgresLegacyStateRejectsUnsupportedOrPartialShape(t *testing.T) {
	if _, err := legacyPostgresState(22, postgresLegacyShape{workspacePath: true, sessionUpdatedAt: true}); err == nil {
		t.Fatal("future legacy marker was accepted")
	}
	if _, err := legacyPostgresState(21, postgresLegacyShape{workspacePath: true}); err == nil {
		t.Fatal("workspace shape without updated_at was accepted")
	}
	if _, err := legacyPostgresState(21, postgresLegacyShape{sessionUpdatedAt: true}); err == nil {
		t.Fatal("updated_at shape without message_file_contexts was accepted")
	}
	if _, err := legacyPostgresState(19, postgresLegacyShape{
		sessionCompactions: true,
		fileVersions:       true,
	}); err == nil {
		t.Fatal("file_versions shape without message_attachments was accepted")
	}
	if _, err := legacyPostgresState(19, postgresLegacyShape{
		sessionCompactions: true,
		channelProvenance:  true,
		cronJobs:           true,
		messageAttachments: true,
		fileVersions:       true,
	}); err == nil {
		t.Fatal("file_versions shape without file_reads was accepted")
	}
}
