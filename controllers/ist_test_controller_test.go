package controllers

import (
	"psikologi_apps/models"
	"testing"
)

func TestShuffleISTQuestionOptions(t *testing.T) {
	qOrig := models.ISTQuestion{
		Id:      77,
		Prompt:  "Jika seorang anak memiliki 50 rupiah...",
		OptionA: "35",
		OptionB: "40",
		OptionC: "45",
		OptionD: "30",
		OptionE: "25",
		Correct: "A",
	}

	q1 := qOrig
	seed1 := int64(101*100000 + q1.Id)
	shuffleISTQuestionOptions(&q1, seed1)

	// Pastikan 5 opsi tetap merupakan himpunan opsi yang sama
	allOpts := map[string]bool{
		q1.OptionA: true,
		q1.OptionB: true,
		q1.OptionC: true,
		q1.OptionD: true,
		q1.OptionE: true,
	}
	expectedOpts := []string{"35", "40", "45", "30", "25"}
	for _, opt := range expectedOpts {
		if !allOpts[opt] {
			t.Errorf("Option %s missing after shuffle", opt)
		}
	}

	// Pastikan opsi yang ditunjuk oleh q1.Correct memiliki nilai "35" (teks jawaban benar semula)
	var correctVal string
	switch q1.Correct {
	case "A":
		correctVal = q1.OptionA
	case "B":
		correctVal = q1.OptionB
	case "C":
		correctVal = q1.OptionC
	case "D":
		correctVal = q1.OptionD
	case "E":
		correctVal = q1.OptionE
	}

	if correctVal != "35" {
		t.Errorf("Expected correct option text to be '35', got '%s' (Correct letter: %s)", correctVal, q1.Correct)
	}

	// Test determinisme: seed yang sama menghasilkan pengacakan yang persis sama
	q2 := qOrig
	shuffleISTQuestionOptions(&q2, seed1)
	if q1.OptionA != q2.OptionA || q1.OptionB != q2.OptionB || q1.OptionC != q2.OptionC ||
		q1.OptionD != q2.OptionD || q1.OptionE != q2.OptionE || q1.Correct != q2.Correct {
		t.Errorf("Deterministic shuffle failed for same seed")
	}

	// Test variabilitas: seed yang berbeda menghasilkan pengacakan yang berbeda (atau minimal diproses ulang secara valid)
	q3 := qOrig
	seed2 := int64(999*100000 + q3.Id)
	shuffleISTQuestionOptions(&q3, seed2)

	var correctVal3 string
	switch q3.Correct {
	case "A":
		correctVal3 = q3.OptionA
	case "B":
		correctVal3 = q3.OptionB
	case "C":
		correctVal3 = q3.OptionC
	case "D":
		correctVal3 = q3.OptionD
	case "E":
		correctVal3 = q3.OptionE
	}

	if correctVal3 != "35" {
		t.Errorf("Expected correct option text to be '35' for seed2, got '%s'", correctVal3)
	}
}
