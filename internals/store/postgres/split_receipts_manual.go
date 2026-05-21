package tabmate

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

type SplitReceipts struct {
	ID               pgtype.UUID        `json:"id"`
	SplitID          pgtype.UUID        `json:"split_id"`
	ObjectKey        string             `json:"object_key"`
	ImageUrl         string             `json:"image_url"`
	MediaType        string             `json:"media_type"`
	OriginalFilename pgtype.Text        `json:"original_filename"`
	CreatedBy        pgtype.UUID        `json:"created_by"`
	CreatedAt        pgtype.Timestamptz `json:"created_at"`
	UpdatedAt        pgtype.Timestamptz `json:"updated_at"`
}

type UpsertSplitReceiptParams struct {
	SplitID          pgtype.UUID `json:"split_id"`
	ObjectKey        string      `json:"object_key"`
	ImageUrl         string      `json:"image_url"`
	MediaType        string      `json:"media_type"`
	OriginalFilename pgtype.Text `json:"original_filename"`
	CreatedBy        pgtype.UUID `json:"created_by"`
}

func (q *Queries) UpsertSplitReceipt(ctx context.Context, arg UpsertSplitReceiptParams) (SplitReceipts, error) {
	row := q.db.QueryRow(ctx, `
INSERT INTO split_receipts (
    split_id,
    object_key,
    image_url,
    media_type,
    original_filename,
    created_by
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (split_id) DO UPDATE SET
    object_key = EXCLUDED.object_key,
    image_url = EXCLUDED.image_url,
    media_type = EXCLUDED.media_type,
    original_filename = EXCLUDED.original_filename,
    updated_at = NOW()
RETURNING id, split_id, object_key, image_url, media_type, original_filename, created_by, created_at, updated_at
`, arg.SplitID, arg.ObjectKey, arg.ImageUrl, arg.MediaType, arg.OriginalFilename, arg.CreatedBy)

	var receipt SplitReceipts
	err := row.Scan(
		&receipt.ID,
		&receipt.SplitID,
		&receipt.ObjectKey,
		&receipt.ImageUrl,
		&receipt.MediaType,
		&receipt.OriginalFilename,
		&receipt.CreatedBy,
		&receipt.CreatedAt,
		&receipt.UpdatedAt,
	)
	return receipt, err
}

func (q *Queries) GetSplitReceiptBySplitID(ctx context.Context, splitID pgtype.UUID) (SplitReceipts, error) {
	row := q.db.QueryRow(ctx, `
SELECT id, split_id, object_key, image_url, media_type, original_filename, created_by, created_at, updated_at
FROM split_receipts
WHERE split_id = $1
`, splitID)

	var receipt SplitReceipts
	err := row.Scan(
		&receipt.ID,
		&receipt.SplitID,
		&receipt.ObjectKey,
		&receipt.ImageUrl,
		&receipt.MediaType,
		&receipt.OriginalFilename,
		&receipt.CreatedBy,
		&receipt.CreatedAt,
		&receipt.UpdatedAt,
	)
	return receipt, err
}
