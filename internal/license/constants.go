// Package license implements the Xiaohualu license service inside Memos.
package license

import "time"

// Frozen V1 business parameters.
//
// The values are the confirmed decisions from
// docs/小花录软件授权服务设计参考文档/实施任务计划.md (T01) and
// 集成到memos实施方案.md §13. Later tasks must reference these constants
// instead of re-declaring the literals, otherwise the contract drifts between
// the client, the HTTP layer, and the database.
const (
	// CardKeyPrefix is the fixed prefix of every card key.
	CardKeyPrefix = "XHL"
	// CardKeySegmentLength is the number of characters in one card key segment.
	CardKeySegmentLength = 4
	// CardKeySegmentCount is the number of segments after the prefix.
	CardKeySegmentCount = 4
	// CardKeyAlphabet omits the ambiguous characters 0, O, 1, I, and L.
	CardKeyAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"
)

// Plan values accepted by the server and embedded in signed tokens.
const (
	// PlanDaily is the day-based plan label.
	PlanDaily = "daily"
	// PlanMonthly is the month-based plan label.
	PlanMonthly = "monthly"
	// PlanYearly is the year-based plan label.
	PlanYearly = "yearly"
	// PlanLifetime is the non-expiring plan label.
	PlanLifetime = "lifetime"
)

// Duration defaults and input bounds.
//
// Confirmed in 实施方案 §13 item 1: durationDays may be any value in the
// supported range, and the plan name is only a label. The defaults below are
// what the UI prefills for the named plans; the API must not reject a custom
// duration for a daily/monthly/yearly card.
const (
	// DefaultDurationDaysDaily is the prefilled duration for the daily plan.
	DefaultDurationDaysDaily = 1
	// DefaultDurationDaysMonthly is the prefilled duration for the monthly plan.
	DefaultDurationDaysMonthly = 30
	// DefaultDurationDaysYearly is the prefilled duration for the yearly plan.
	DefaultDurationDaysYearly = 365
	// LifetimeDurationDays is the only valid duration for a lifetime card.
	LifetimeDurationDays = 0
	// MinDurationDays is the smallest accepted duration for a non-lifetime card.
	MinDurationDays = 1
	// MaxDurationDays is the largest accepted duration for a non-lifetime card.
	MaxDurationDays = 3650
)

// Admin input bounds.
const (
	// MinCreateCount is the smallest number of cards created in one request.
	MinCreateCount = 1
	// MaxCreateCount is the largest number of cards created in one request.
	MaxCreateCount = 1000
	// MinPageSize is the smallest accepted page size.
	MinPageSize = 1
	// MaxPageSize is the largest accepted page size.
	MaxPageSize = 200
)

// HTTP routes.
const (
	// ClientRoutePrefix is the base path configured in the desktop client.
	ClientRoutePrefix = "/xiaohualu"
	// ClientAPIPrefix is the prefix of the client contract endpoints.
	ClientAPIPrefix = ClientRoutePrefix + "/v1"
	// AdminAPIPrefix is the prefix of the admin license endpoints.
	AdminAPIPrefix = "/admin/api/license"
)

// Request and rate-limiting defaults.
const (
	// MaxRequestBytes caps the JSON body of client endpoints (64 KiB).
	MaxRequestBytes = 64 << 10
	// DefaultRateLimitPerMinute is the per-IP client request budget.
	DefaultRateLimitPerMinute = 10
	// DefaultUpdateMaxBytes caps an uploaded installer (512 MiB).
	DefaultUpdateMaxBytes = 512 << 20
)

// Token parameters.
const (
	// TokenVersion is the only accepted token payload version.
	TokenVersion = 1
	// TokenNonceBytes is the size of the random nonce in every signed token.
	TokenNonceBytes = 16
)

// TimeLayout is the RFC3339 UTC text format used for database timestamps,
// admin responses, and the absolute licence expiry input.
const TimeLayout = time.RFC3339
