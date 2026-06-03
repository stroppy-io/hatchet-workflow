package quotas

import "context"

func (s *Store) ListYandexTenantIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.Pool.Query(ctx, `
select tenant_id
from tenant_settings_records
where coalesce(data->'yandexSettings'->>'token', '') <> ''
  and coalesce(data->'yandexSettings'->>'cloudId', '') <> ''
order by tenant_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var tenantID string
		if err := rows.Scan(&tenantID); err != nil {
			return nil, err
		}
		out = append(out, tenantID)
	}
	return out, rows.Err()
}
