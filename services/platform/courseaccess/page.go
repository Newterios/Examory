package courseaccess

import "context"

// Visible fills a keyset page after authorization, caching repeated scope checks for this request only.
func Visible[T any](ctx context.Context, limit int, after string, fetch func(string, int) ([]T, error), scope func(T) (string, string, string), allowed func(string, string) (bool, error)) ([]T, error) {
	var out []T
	cache := make(map[[2]string]bool)
	for len(out) < limit {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		batch, err := fetch(after, limit)
		if err != nil {
			return nil, err
		}
		for _, row := range batch {
			id, course, group := scope(row)
			after = id
			key := [2]string{course, group}
			ok, seen := cache[key]
			if !seen {
				ok, err = allowed(course, group)
				if err != nil {
					return nil, err
				}
				cache[key] = ok
			}
			if ok {
				out = append(out, row)
				if len(out) == limit {
					return out, nil
				}
			}
		}
		if len(batch) < limit {
			break
		}
	}
	return out, nil
}
