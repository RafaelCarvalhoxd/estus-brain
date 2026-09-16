package domain

import "time"

type PaymentMethod string

const (
	PaymentDebit  PaymentMethod = "debito"
	PaymentCredit PaymentMethod = "credito"
	PaymentPix    PaymentMethod = "pix"
)

func (p PaymentMethod) Valid() bool {
	switch p {
	case PaymentDebit, PaymentCredit, PaymentPix:
		return true
	}
	return false
}

// Transaction is a single ledger entry. A parceled purchase becomes N
// Transaction rows sharing InstallmentGroupID, each carrying its own
// CompetenceMonth — an installment is not "one purchase with a counter", it
// is N independent expenses that happen to originate from one purchase, and
// the dashboard must be able to sum any single month correctly without
// knowing about the others.
type Transaction struct {
	ID                 string
	Description        string
	AmountCents        Cents
	CategoryID         string
	PaymentMethod      PaymentMethod
	PurchaseDate       time.Time
	CreditCardID       *string
	CompetenceMonth    YearMonth
	InstallmentGroupID *string
	InstallmentNumber  int // 1-based; 0 when not an installment
	InstallmentTotal   int // 1 when not an installment
	IsRecurring        bool
	CreatedAt          time.Time
}

// NewInstallmentPurchase builds the N transactions a parceled credit-card
// purchase expands into. total is the full purchase amount; installments is
// how many months it's split across (1 means "not parceled").
func NewInstallmentPurchase(desc string, total Cents, categoryID string, card CreditCard, purchaseDate time.Time, installments int) []Transaction {
	if installments < 1 {
		installments = 1
	}
	firstCompetence := CompetenceMonth(purchaseDate, PaymentCredit, &card)
	parts := total.Split(installments)

	var groupID *string
	if installments > 1 {
		id := newID()
		groupID = &id
	}

	txns := make([]Transaction, installments)
	for i := 0; i < installments; i++ {
		txns[i] = Transaction{
			ID:                 newID(),
			Description:        desc,
			AmountCents:        parts[i],
			CategoryID:         categoryID,
			PaymentMethod:      PaymentCredit,
			PurchaseDate:       purchaseDate,
			CreditCardID:       &card.ID,
			CompetenceMonth:    firstCompetence.Add(i),
			InstallmentGroupID: groupID,
			InstallmentNumber:  i + 1,
			InstallmentTotal:   installments,
		}
	}
	return txns
}
