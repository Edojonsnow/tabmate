-- name: UpsertSplitReceipt :one
INSERT INTO split_receipts (
    split_id,
    object_key,
    image_url,
    media_type,
    original_filename,
    created_by
) VALUES (@split_id, @object_key, @image_url, @media_type, @original_filename, @created_by)
ON CONFLICT (split_id) DO UPDATE SET
    object_key = EXCLUDED.object_key,
    image_url = EXCLUDED.image_url,
    media_type = EXCLUDED.media_type,
    original_filename = EXCLUDED.original_filename,
    updated_at = NOW()
RETURNING id, split_id, object_key, image_url, media_type, original_filename, created_by, created_at, updated_at;

-- name: GetSplitReceiptBySplitID :one
SELECT id, split_id, object_key, image_url, media_type, original_filename, created_by, created_at, updated_at
FROM split_receipts
WHERE split_id = $1;
