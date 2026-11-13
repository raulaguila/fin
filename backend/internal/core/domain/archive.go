package domain

// Archive is everything the user set up, in one JSON file: what a new install
// needs besides the bank data the next sync brings back. It is a file format,
// so its JSON names live here, next to the version that guards them.
// Categories, rules and every other reference go by name, never by id.
type Archive struct {
	Version       int                   `json:"version"`
	ExportedAt    string                `json:"exported_at"`
	Categories    []ArchiveCategory     `json:"categories,omitempty"`
	Rules         []ArchiveRule         `json:"rules,omitempty"`
	Goals         []ArchiveGoal         `json:"goals,omitempty"`
	Subscriptions []ArchiveSubscription `json:"subscriptions,omitempty"`
	Debts         []ArchiveDebt         `json:"debts,omitempty"`
	Accounts      []ArchiveAccount      `json:"accounts,omitempty"`
	Overrides     []ArchiveOverride     `json:"overrides,omitempty"`
}

// ArchiveVersion is the format this build writes and reads.
const ArchiveVersion = 1

type ArchiveCategory struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Limit    Money  `json:"limit"`
	Rollover bool   `json:"rollover"`
}

type ArchiveRule struct {
	Keyword  string `json:"keyword"`
	Category string `json:"category"`
}

type ArchiveGoal struct {
	Name    string `json:"name"`
	Target  Money  `json:"target"`
	Saved   Money  `json:"saved"`
	DueDate string `json:"due_date,omitempty"` // YYYY-MM-DD
}

type ArchiveSubscription struct {
	Name     string `json:"name"`
	Amount   Money  `json:"amount"`
	Day      int    `json:"day"`
	Month    int    `json:"month"`
	Keyword  string `json:"keyword"`
	Category string `json:"category,omitempty"`
}

type ArchiveDebt struct {
	Person      string `json:"person"`
	Amount      Money  `json:"amount"`
	Description string `json:"description,omitempty"`
	DueDate     string `json:"due_date,omitempty"`
	SettledOn   string `json:"settled_on,omitempty"`
}

// ArchiveAccount is an account with no bank behind it, with its transactions:
// nothing else could bring them back.
type ArchiveAccount struct {
	Institution  string               `json:"institution"`
	Name         string               `json:"name"`
	Type         string               `json:"type"`
	Transactions []ArchiveTransaction `json:"transactions,omitempty"`
}

type ArchiveTransaction struct {
	// Seq is the position in the exported account; a re-sent part skips what is already in.
	Seq         int      `json:"seq"`
	Date        string   `json:"date"`
	Description string   `json:"description"`
	Amount      Money    `json:"amount"`
	Category    string   `json:"category,omitempty"`
	Note        string   `json:"note,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// ArchiveOverride is what the user changed on a bank transaction, found again
// by the bank's id after the sync brings it back.
type ArchiveOverride struct {
	ExternalID   string   `json:"external_id"`
	Category     string   `json:"category,omitempty"`
	UserCategory bool     `json:"user_category"` // the category was the user's choice
	Note         string   `json:"note,omitempty"`
	Tags         []string `json:"tags,omitempty"`
}

// ArchiveResult counts what an import added or changed.
type ArchiveResult struct {
	Categories    int   `json:"categories"`
	Rules         int   `json:"rules"`
	Goals         int   `json:"goals"`
	Subscriptions int   `json:"subscriptions"`
	Debts         int   `json:"debts"`
	Accounts      int   `json:"accounts"`
	Transactions  int64 `json:"transactions"`
	Overrides     int64 `json:"overrides"`
}
