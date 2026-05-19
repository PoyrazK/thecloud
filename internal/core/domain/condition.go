package domain

// ConditionOperator defines the operator used in a condition evaluation.
// Follows AWS IAM condition operator naming conventions.
type ConditionOperator string

const (
	// IP-based operators
	CondIpAddress    ConditionOperator = "IpAddress"
	CondNotIpAddress ConditionOperator = "NotIpAddress"

	// String-based operators
	CondStringEquals    ConditionOperator = "StringEquals"
	CondStringNotEquals ConditionOperator = "StringNotEquals"
	CondStringLike      ConditionOperator = "StringLike"
	CondStringNotLike   ConditionOperator = "StringNotLike"

	// Date-based operators
	CondDateGreaterThan ConditionOperator = "DateGreaterThan"
	CondDateLessThan    ConditionOperator = "DateLessThan"
	CondDateEquals      ConditionOperator = "DateEquals"

	// Boolean operator
	CondBool ConditionOperator = "Bool"

	// Null check operator
	CondNull ConditionOperator = "Null"
)

// ConditionKey represents well-known condition keys used in evaluation context.
type ConditionKey string

const (
	// thecloud condition keys
	KeySourceIP      ConditionKey = "thecloud:SourceIp"
	KeyUserID        ConditionKey = "thecloud:UserId"
	KeyUsername      ConditionKey = "thecloud:Username"
	KeyCurrentTime   ConditionKey = "thecloud:CurrentTime"
	KeyRequestedTime ConditionKey = "thecloud:RequestedTime"
	KeyUserAgent     ConditionKey = "thecloud:UserAgent"
	KeyTenantID      ConditionKey = "thecloud:TenantId"
)
