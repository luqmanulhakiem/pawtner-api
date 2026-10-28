package models

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	PeriodPhaseUnknown       = "unknown"
	PeriodPhaseMenstruation  = "menstruation"
	PeriodPhaseFollicular    = "follicular"
	PeriodPhaseOvulation     = "ovulation"
	PeriodPhaseLuteal        = "luteal"
	PeriodPhasePremenstrual  = "premenstrual"
	PeriodPhaseLate          = "late"
	PeriodSeverityNormal     = "normal"
	PeriodSeverityAttention  = "attention"
	PeriodSeverityConsultDoc = "consult_doctor"

	defaultCycleLength  = 28
	defaultPeriodLength = 5
	lutealPhaseLength   = 14 // jarak ovulasi ke haid berikutnya relatif stabil (~14 hari)
	insightHistoryLimit = 6  // jumlah siklus terakhir yang dipakai untuk rata-rata
	moodLookbackDays    = 7
)

// emotionalMoods adalah nilai DailyMood.MoodStatus yang dianggap emosional.
// "sick" tidak dihitung karena menunjukkan kondisi fisik, bukan emosi.
var emotionalMoods = map[string]bool{
	MoodStatusSad:   true,
	MoodStatusAngry: true,
}

type MoodSignal struct {
	CheckedDays  int  `json:"checked_days"`  // jumlah hari yang punya catatan mood
	NegativeDays int  `json:"negative_days"` // jumlah hari dengan mood emosional
	Emotional    bool `json:"emotional"`
}

type FertileWindow struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// PeriodInsight adalah gambaran fase siklus user pada tanggal tertentu.
type PeriodInsight struct {
	Date                string         `json:"date"`
	Phase               string         `json:"phase"`
	Severity            string         `json:"severity"`
	Title               string         `json:"title"`
	Description         string         `json:"description"`
	Recommendations     []string       `json:"recommendations"`
	CycleDay            int            `json:"cycle_day"`
	LastPeriodStart     *string        `json:"last_period_start"`
	ExpectedNextPeriod  *string        `json:"expected_next_period"`
	DaysUntilNextPeriod *int           `json:"days_until_next_period"` // negatif jika sudah lewat
	LateDays            int            `json:"late_days"`
	AverageCycleLength  int            `json:"average_cycle_length"`
	AveragePeriodLength int            `json:"average_period_length"`
	CyclesAnalyzed      int            `json:"cycles_analyzed"` // 0 berarti memakai nilai default
	FertileWindow       *FertileWindow `json:"fertile_window"`
	MoodSignal          MoodSignal     `json:"mood_signal"`
}

func parseDay(value string) (time.Time, error) {
	return time.Parse("2006-01-02", value)
}

func formatDay(t time.Time) string {
	return t.Format("2006-01-02")
}

func daysBetween(from, to time.Time) int {
	return int(to.Sub(from).Hours() / 24)
}

func plural(n int, unit string) string {
	return fmt.Sprintf("%d %s", n, unit)
}

// humanizeDays mengubah jumlah hari menjadi "N hari", "N minggu", atau "N bulan".
func humanizeDays(days int) string {
	switch {
	case days >= 60:
		return plural(days/30, "bulan")
	case days >= 14:
		return plural(days/7, "minggu")
	default:
		return plural(days, "hari")
	}
}

func isNegativeMood(status string) bool {
	return emotionalMoods[strings.ToLower(strings.TrimSpace(status))]
}

func moodSignal(db *gorm.DB, userId uint, date time.Time) (MoodSignal, error) {
	var moods []DailyMood
	from := formatDay(date.AddDate(0, 0, -(moodLookbackDays - 1)))
	if err := db.Where("user_id = ? AND date BETWEEN ? AND ?", userId, from, formatDay(date)).Find(&moods).Error; err != nil {
		return MoodSignal{}, err
	}

	checked, negative := map[string]bool{}, map[string]bool{}
	for _, mood := range moods {
		checked[mood.Date] = true
		if isNegativeMood(mood.MoodStatus) {
			negative[mood.Date] = true
		}
	}

	signal := MoodSignal{CheckedDays: len(checked), NegativeDays: len(negative)}
	signal.Emotional = signal.NegativeDays >= 2
	return signal, nil
}

// cycleStats menghitung rata-rata panjang siklus (selisih start_date berurutan) dan
// panjang haid dari riwayat. Selisih di luar 15-60 hari dianggap outlier (mis. ada
// bulan yang tidak tercatat) dan diabaikan. periods harus urut dari lama ke baru.
func cycleStats(periods []Period) (cycleLen, periodLen, cycles int) {
	var gaps []int
	for i := 1; i < len(periods); i++ {
		prev, _ := parseDay(periods[i-1].StartDate)
		cur, _ := parseDay(periods[i].StartDate)
		if gap := daysBetween(prev, cur); gap >= 15 && gap <= 60 {
			gaps = append(gaps, gap)
		}
	}
	cycleLen = averageLast(gaps, defaultCycleLength)

	var lengths []int
	for _, p := range periods {
		if p.EndDate == nil {
			continue
		}
		start, _ := parseDay(p.StartDate)
		end, _ := parseDay(*p.EndDate)
		if n := daysBetween(start, end) + 1; n >= 2 && n <= 10 {
			lengths = append(lengths, n)
		}
	}
	return cycleLen, averageLast(lengths, defaultPeriodLength), len(gaps)
}

func averageLast(values []int, fallback int) int {
	if len(values) == 0 {
		return fallback
	}
	if len(values) > insightHistoryLimit {
		values = values[len(values)-insightHistoryLimit:]
	}
	sum := 0
	for _, v := range values {
		sum += v
	}
	return (sum + len(values)/2) / len(values)
}

// GetPeriodInsight menyusun deskripsi fase siklus user female pada date dari riwayat
// period, perhitungan umum siklus, dan riwayat daily mood.
func GetPeriodInsight(db *gorm.DB, userId uint, dateStr string) (PeriodInsight, error) {
	insight := PeriodInsight{Date: dateStr, Phase: PeriodPhaseUnknown, Severity: PeriodSeverityNormal, Recommendations: []string{}}

	if err := requireGender(db, userId, GenderFemale, ErrPeriodFemaleOnly); err != nil {
		return insight, err
	}
	date, err := parseDay(dateStr)
	if err != nil {
		return insight, err
	}

	// Hanya period yang sudah dimulai pada/sebelum date yang relevan
	var periods []Period
	if err := db.Where("user_id = ? AND start_date <= ?", userId, dateStr).Order("start_date ASC").Find(&periods).Error; err != nil {
		return insight, err
	}

	signal, err := moodSignal(db, userId, date)
	if err != nil {
		return insight, err
	}
	insight.MoodSignal = signal

	cycleLen, periodLen, cycles := cycleStats(periods)
	insight.AverageCycleLength, insight.AveragePeriodLength, insight.CyclesAnalyzed = cycleLen, periodLen, cycles

	if len(periods) == 0 {
		insight.Title = "Belum ada data haid"
		insight.Description = "Belum ada catatan haid sampai tanggal ini, sehingga fase siklus belum bisa dihitung."
		insight.Recommendations = []string{"Catat tanggal mulai haid terakhirmu agar prediksi siklus bisa dibuat."}
		return insight, nil
	}

	last := periods[len(periods)-1]
	lastStart, _ := parseDay(last.StartDate)
	expectedNext := lastStart.AddDate(0, 0, cycleLen)
	day := daysBetween(lastStart, date) + 1
	until := daysBetween(date, expectedNext)

	// Ovulasi ~14 hari sebelum haid berikutnya, tetapi tidak boleh terlalu dekat dengan haid
	ovulation := cycleLen - lutealPhaseLength
	if ovulation < periodLen+2 {
		ovulation = periodLen + 2
	}
	premenstrualStart := cycleLen - 4
	if premenstrualStart < ovulation+2 {
		premenstrualStart = ovulation + 2
	}
	ovulationDate := lastStart.AddDate(0, 0, ovulation-1)

	lastStartStr, expectedStr := last.StartDate, formatDay(expectedNext)
	insight.CycleDay = day
	insight.LastPeriodStart = &lastStartStr
	insight.ExpectedNextPeriod = &expectedStr
	insight.DaysUntilNextPeriod = &until
	insight.FertileWindow = &FertileWindow{
		Start: formatDay(ovulationDate.AddDate(0, 0, -5)),
		End:   formatDay(ovulationDate.AddDate(0, 0, 1)),
	}

	onPeriod := last.EndDate == nil || *last.EndDate >= dateStr

	switch {
	case onPeriod:
		describeMenstruation(&insight, periods, day, periodLen)
	case day > cycleLen:
		describeLate(&insight, day-cycleLen)
	case day >= premenstrualStart:
		describePremenstrual(&insight, until)
	case day > ovulation+1:
		insight.Phase = PeriodPhaseLuteal
		insight.Title = "Fase luteal (normal)"
		insight.Description = fmt.Sprintf("Ovulasi sudah lewat dan tubuh berada di fase luteal. Kondisi ini normal; haid berikutnya diperkirakan sekitar %s (%d hari lagi).", expectedStr, until)
		insight.Recommendations = []string{"Jaga pola tidur dan makan teratur.", "Perhatikan perubahan mood atau gejala fisik, bisa dicatat di daily mood."}
	case day >= ovulation-1:
		insight.Phase = PeriodPhaseOvulation
		insight.Title = "Fase ovulasi"
		insight.Description = "Kamu diperkirakan sedang masa ovulasi (masa subur). Sebagian orang merasakan lendir serviks lebih bening dan licin, nyeri ringan di satu sisi perut, atau energi lebih tinggi."
		insight.Recommendations = []string{"Ini adalah perkiraan berdasarkan rata-rata siklusmu, bukan alat kontrasepsi.", "Jika sedang program hamil, ini waktu yang paling subur."}
	default:
		insight.Phase = PeriodPhaseFollicular
		insight.Title = "Fase folikular (setelah haid)"
		insight.Description = "Haid sudah selesai dan tubuh bersiap menuju ovulasi. Kadar estrogen mulai naik sehingga energi dan mood biasanya membaik."
		insight.Recommendations = []string{"Waktu yang baik untuk aktivitas fisik yang lebih intens.", fmt.Sprintf("Perkiraan ovulasi sekitar %s.", formatDay(ovulationDate))}
	}

	return insight, nil
}

func describeMenstruation(in *PeriodInsight, periods []Period, day, periodLen int) {
	in.Phase = PeriodPhaseMenstruation
	in.Title = "Sedang haid"
	in.Description = fmt.Sprintf("Hari ke-%d haid (rata-rata haidmu %d hari). Nyeri perut, lelah, dan mood naik turun adalah hal umum di fase ini.", day, periodLen)
	in.Recommendations = []string{"Istirahat cukup dan minum air putih yang banyak.", "Kompres hangat di perut bisa membantu meredakan nyeri."}
	if day > 10 {
		in.Severity = PeriodSeverityConsultDoc
		in.Description = fmt.Sprintf("Haid sudah berlangsung %d hari, lebih lama dari batas umum (sekitar 7 hari).", day)
		in.Recommendations = append(in.Recommendations, "Haid yang berlangsung lebih dari 7-10 hari sebaiknya diperiksakan ke dokter.")
		return
	}

	// Bandingkan siklus yang baru saja dimulai dengan kebiasaan sebelumnya
	if len(periods) < 2 {
		return
	}
	latest, _ := parseDay(periods[len(periods)-1].StartDate)
	previous, _ := parseDay(periods[len(periods)-2].StartDate)
	gap := daysBetween(previous, latest)
	baseline, _, _ := cycleStats(periods[:len(periods)-1])

	switch {
	case gap < 21:
		in.Severity = PeriodSeverityAttention
		in.Description += fmt.Sprintf(" Haid kali ini datang setelah %d hari, lebih cepat dari rentang normal siklus (21-35 hari).", gap)
		in.Recommendations = append(in.Recommendations,
			"Siklus pendek sesekali bisa dipicu stres, kurang tidur, olahraga berlebihan, atau perubahan berat badan.",
			"Jika terjadi 2-3 siklus berturut-turut atau disertai perdarahan banyak, periksakan ke dokter.",
			"Perdarahan ringan yang datang lebih awal juga bisa bukan haid sebenarnya (misalnya bercak); perhatikan jumlah darahnya.")
	case baseline-gap >= 4:
		early := baseline - gap
		in.Severity = PeriodSeverityAttention
		in.Description += fmt.Sprintf(" Haid kali ini datang %d hari lebih cepat dari biasanya (rata-rata siklusmu %d hari, kali ini %d hari).", early, baseline, gap)
		in.Recommendations = append(in.Recommendations,
			"Selisih beberapa hari masih bisa wajar; stres, jadwal tidur, dan perubahan aktivitas dapat memengaruhi.",
			"Jika sering lebih cepat, catat polanya dan konsultasikan ke dokter.")
	}
}

func describePremenstrual(in *PeriodInsight, until int) {
	in.Phase = PeriodPhasePremenstrual
	in.Title = "Menjelang haid (fase PMS)"
	when := fmt.Sprintf("sekitar %d hari lagi", until)
	if until <= 0 {
		when = "hari ini atau besok"
	}
	in.Description = fmt.Sprintf("Haid diperkirakan datang %s. Penurunan hormon di fase ini bisa memicu mudah emosi, perut kembung, payudara nyeri, dan ingin makan lebih banyak.", when)
	in.Recommendations = []string{"Kurangi kafein dan makanan terlalu asin/manis.", "Tidur cukup dan siapkan perlengkapan haid."}
	if in.MoodSignal.Emotional {
		in.Description += fmt.Sprintf(" Catatan mood-mu menunjukkan emosi yang kurang stabil (%d dari %d hari terakhir), sejalan dengan gejala PMS.", in.MoodSignal.NegativeDays, in.MoodSignal.CheckedDays)
		in.Recommendations = append(in.Recommendations, "Wajar jika sedang lebih sensitif; beri waktu untuk diri sendiri dan komunikasikan ke pasangan.")
	}
}

func describeLate(in *PeriodInsight, late int) {
	in.Phase = PeriodPhaseLate
	in.LateDays = late
	span := humanizeDays(late)
	in.Title = fmt.Sprintf("Haid terlambat %s", span)

	stress := ""
	if in.MoodSignal.Emotional {
		stress = " Catatan mood-mu juga menunjukkan emosi yang kurang stabil akhir-akhir ini, stres bisa jadi salah satu penyebabnya."
	}

	switch {
	case late < 7:
		in.Severity = PeriodSeverityAttention
		in.Description = fmt.Sprintf("Haid terlambat %s dari perkiraan. Keterlambatan beberapa hari masih umum terjadi karena stres, kurang tidur, perubahan pola makan, atau aktivitas fisik.%s", span, stress)
		in.Recommendations = []string{
			"Tunggu beberapa hari lagi sambil mengelola stres dan tidur.",
			"Jika ada kemungkinan hamil, tes kehamilan paling akurat dilakukan setelah haid terlambat sekitar 1 minggu.",
		}
	case late < 14:
		in.Severity = PeriodSeverityAttention
		in.Description = fmt.Sprintf("Haid sudah terlambat sekitar %s. Penyebabnya bisa kehamilan, stres, perubahan berat badan, atau gangguan hormonal.%s", span, stress)
		in.Recommendations = []string{
			"Lakukan tes kehamilan (test pack) dengan urine pagi, terutama jika aktif secara seksual.",
			"Jika hasil negatif dan haid belum datang, ulangi tes beberapa hari kemudian.",
		}
	case late < 60:
		in.Severity = PeriodSeverityConsultDoc
		in.Description = fmt.Sprintf("Haid sudah terlambat sekitar %s. Keterlambatan selama ini perlu dicari penyebabnya: kehamilan, stres berkepanjangan, atau kondisi medis seperti gangguan tiroid dan PCOS.%s", span, stress)
		in.Recommendations = []string{
			"Lakukan tes kehamilan segera.",
			"Jadwalkan pemeriksaan ke dokter atau bidan untuk pemeriksaan hormon dan USG bila perlu.",
			"Segera ke fasilitas kesehatan jika disertai nyeri perut hebat atau perdarahan tidak normal.",
		}
	default:
		in.Severity = PeriodSeverityConsultDoc
		in.Description = fmt.Sprintf("Haid sudah terlambat sekitar %s. Tidak haid selama 3 bulan atau lebih (amenore) memerlukan pemeriksaan medis.%s", span, stress)
		in.Recommendations = []string{
			"Segera konsultasikan ke dokter kandungan.",
			"Lakukan tes kehamilan, dan pemeriksaan untuk menyingkirkan penyakit atau gangguan hormon (tiroid, PCOS, prolaktin).",
			"Kondisi stres berat atau penurunan berat badan drastis juga bisa menjadi penyebab dan perlu ditangani.",
		}
	}
}
