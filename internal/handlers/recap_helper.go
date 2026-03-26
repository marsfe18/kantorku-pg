package handlers

import "kantorku/internal/recap"

// TeamRecapData — data rekap yang sudah dikelompokkan per tim, dikirim ke template
type TeamReqRecap struct {
	Team    string
	Label   string
	Rows    []*recap.RequestRecapRow
	Total   int
}

type TeamStockRecap struct {
	Team  string
	Label string
	Rows  []*recap.StockRecapRow
}

func groupReqRecapByTeam(data []*recap.RequestRecapRow) []TeamReqRecap {
	teamOrder := []struct{ key, label string }{
		{"produksi", "Produksi"},
		{"distribusi", "Distribusi"},
		{"ipds", "IPDS"},
		{"sosial", "Sosial"},
		{"neraca", "Neraca"},
	}

	result := []TeamReqRecap{}
	for _, t := range teamOrder {
		var rows []*recap.RequestRecapRow
		total := 0
		for _, r := range data {
			if r.Team == t.key {
				rows = append(rows, r)
				total += r.Total
			}
		}
		result = append(result, TeamReqRecap{
			Team:  t.key,
			Label: t.label,
			Rows:  rows,
			Total: total,
		})
	}
	return result
}

func groupStockRecapByTeam(data []*recap.StockRecapRow) []TeamStockRecap {
	teamOrder := []struct{ key, label string }{
		{"produksi", "Produksi"},
		{"distribusi", "Distribusi"},
		{"ipds", "IPDS"},
		{"sosial", "Sosial"},
		{"neraca", "Neraca"},
	}

	result := []TeamStockRecap{}
	for _, t := range teamOrder {
		var rows []*recap.StockRecapRow
		for _, r := range data {
			if r.Team == t.key {
				rows = append(rows, r)
			}
		}
		result = append(result, TeamStockRecap{
			Team:  t.key,
			Label: t.label,
			Rows:  rows,
		})
	}
	return result
}
