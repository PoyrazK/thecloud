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
	// Standard AWS condition keys
	KeySourceIP      ConditionKey = "aws:SourceIp"
	KeyUserID        ConditionKey = "aws:UserId"
	KeyUsername      ConditionKey = "aws:Username"
	KeyCurrentTime   ConditionKey = "aws:CurrentTime"
	KeyRequestedTime ConditionKey = "aws:RequestedTime"

	// thecloud-specific condition keys
	KeyTenantID  ConditionKey = "thecloud:TenantId"
	KeyUserAgent ConditionKey = "thecloud:UserAgent"
)
