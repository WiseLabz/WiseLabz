package pbs

import (
	"github.com/WiseLabz/wiselabz/internal/connector"
)

var guestAttributes = []connector.AttributeSpec{
	{Name: "datastore", Type: "string", Description: "Datastore name where the backup group exists"},
	{Name: "namespace", Type: "string", Description: "Namespace within the datastore (empty for root)"},
	{Name: "backup_count", Type: "number", Description: "Total number of backups for this guest across all datastores and namespaces"},
	{Name: "last_backup_age_days", Type: "number", Description: "Whole days since the most recent backup, computed when the connector syncs"},
	{Name: "verify_state", Type: "string", Description: "Verification state of the most recent backup: ok, failed, or none if no verification"},
}

var attributeCatalog = map[string][]connector.AttributeSpec{
	"datastore": {
		{Name: "backend_type", Type: "string", Description: "Storage backend type (filesystem or other)"},
		{Name: "comment", Type: "string", Description: "Datastore comment"},
	},
	"vm":        guestAttributes,
	"container": guestAttributes,
	"host":      guestAttributes,
	"verify_job": {
		{Name: "datastore", Type: "string", Description: "Datastore the job is configured for"},
		{Name: "namespace", Type: "string", Description: "Namespace the job is configured for (empty for root)"},
		{Name: "schedule", Type: "string", Description: "Job schedule as a cron expression"},
		{Name: "ignore_verified", Type: "boolean", Description: "Whether to skip already-verified backups"},
		{Name: "outdated_after_days", Type: "number", Description: "Number of days before a backup is considered outdated"},
		{Name: "max_depth", Type: "number", Description: "Maximum depth of verification recursion"},
	},
	"prune_job": {
		{Name: "datastore", Type: "string", Description: "Datastore the job is configured for"},
		{Name: "namespace", Type: "string", Description: "Namespace the job is configured for (empty for root)"},
		{Name: "schedule", Type: "string", Description: "Job schedule as a cron expression"},
		{Name: "enabled", Type: "boolean", Description: "Whether the job is enabled"},
		{Name: "max_depth", Type: "number", Description: "Maximum depth of pruning recursion"},
		{Name: "keep_last", Type: "number", Description: "Number of most recent backups to keep"},
		{Name: "keep_hourly", Type: "number", Description: "Number of hourly backups to keep"},
		{Name: "keep_daily", Type: "number", Description: "Number of daily backups to keep"},
		{Name: "keep_weekly", Type: "number", Description: "Number of weekly backups to keep"},
		{Name: "keep_monthly", Type: "number", Description: "Number of monthly backups to keep"},
		{Name: "keep_yearly", Type: "number", Description: "Number of yearly backups to keep"},
	},
}
