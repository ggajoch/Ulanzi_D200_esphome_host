package main

import (
	"reflect"
	"testing"
)

func TestParseForecast(t *testing.T) {
	rows, box := parseForecast("05:00|weather-rainy|6°|1,8|11/20;09:00|weather-cloudy|5°||14/26;bad#pada do 20:00|1,8 mm|#4499FF")
	wantRows := [][]string{{"05:00", "weather-rainy", "6°", "1,8", "11/20"}, {"09:00", "weather-cloudy", "5°", "", "14/26"}}
	if !reflect.DeepEqual(rows, wantRows) {
		t.Errorf("rows = %q, want %q", rows, wantRows)
	}
	if want := []string{"pada do 20:00", "1,8 mm", "#4499FF"}; !reflect.DeepEqual(box, want) {
		t.Errorf("box = %q, want %q", box, want)
	}
	if rows, box := parseForecast(""); rows != nil || box != nil {
		t.Errorf("empty value: rows %q, box %q", rows, box)
	}
}
