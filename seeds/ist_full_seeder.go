package seeds

import (
	"fmt"
	"strings"

	"psikologi_apps/models"

	"github.com/beego/beego/v2/client/orm"
)

// SeedISTFull membuat data lengkap untuk IST berdasarkan soal yang diberikan
func SeedISTFull() error {
	o := orm.NewOrm()

	// Pastikan subtests sudah ada
	subtests := []struct {
		Code       string
		Name       string
		OrderIndex int
	}{
		{"SE", "Sentence Completion", 1},
		{"WA", "Word Analogies", 2},
		{"AN", "Analogies", 3},
		{"GE", "General Comprehension", 4},
		{"RA", "Arithmetic", 5},
		{"ZR", "Number Series", 6},
		{"FA", "Shape Assembly", 7},
		{"WU", "Cube Rotation", 8},
		{"ME", "Memory", 9},
	}

	subtestMap := make(map[string]*models.ISTSubtest)
	for _, s := range subtests {
		var sub models.ISTSubtest
		err := o.QueryTable(new(models.ISTSubtest)).Filter("Code", s.Code).One(&sub)
		if err != nil {
			sub = models.ISTSubtest{Code: s.Code, Name: s.Name, OrderIndex: s.OrderIndex}
			if _, err := o.Insert(&sub); err != nil {
				return fmt.Errorf("insert subtest %s: %w", s.Code, err)
			}
		} else {
			// Pastikan name & orderIndex sesuai paket IST 176 soal
			needUpdate := false
			if sub.Name != s.Name {
				sub.Name = s.Name
				needUpdate = true
			}
			if sub.OrderIndex != s.OrderIndex {
				sub.OrderIndex = s.OrderIndex
				needUpdate = true
			}
			if needUpdate {
				if _, uerr := o.Update(&sub, "Name", "OrderIndex"); uerr != nil {
					return fmt.Errorf("update subtest %s: %w", s.Code, uerr)
				}
			}
		}
		subtestMap[s.Code] = &sub
	}

	// Nonaktifkan subtest lain (mis. ZA dari seeder trial lama) agar tidak mengganggu urutan.
	var allSubs []models.ISTSubtest
	_, _ = o.QueryTable(new(models.ISTSubtest)).All(&allSubs)
	allowed := map[string]bool{
		"SE": true, "WA": true, "AN": true, "GE": true, "RA": true, "ZR": true, "FA": true, "WU": true, "ME": true,
	}
	for _, s := range allSubs {
		if allowed[s.Code] {
			continue
		}
		// Dorong ke belakang supaya tidak ikut flow
		if s.OrderIndex < 900 {
			s.OrderIndex = 999
			_, _ = o.Update(&s, "OrderIndex")
		}
	}

	fmt.Println("Starting IST full seeder...")
	
	// SE (01-20): Sentence Completion
	fmt.Println("Seeding SE (Sentence Completion)...")
	seedSE(o, subtestMap["SE"])

	// WA (21-40): Word Analogies
	fmt.Println("Seeding WA (Word Analogies)...")
	seedWA(o, subtestMap["WA"])

	// AN (41-60): Analogies
	fmt.Println("Seeding AN (Analogies)...")
	seedAN(o, subtestMap["AN"])

	// GE (61-76): General Comprehension (jawaban teks bebas, disimpan sebagai pilihan ganda)
	fmt.Println("Seeding GE (General Comprehension)...")
	seedGE(o, subtestMap["GE"])

	// RA (77-96): Arithmetic
	fmt.Println("Seeding RA (Arithmetic)...")
	seedRA(o, subtestMap["RA"])

	// ZR (97-116): Number Series
	fmt.Println("Seeding ZR (Number Series)...")
	seedZR(o, subtestMap["ZR"])

	// FA (117-136): Shape Assembly (dengan gambar)
	fmt.Println("Seeding FA (Shape Assembly)...")
	seedFA(o, subtestMap["FA"])

	// WU (137-156): Cube Rotation (dengan gambar)
	fmt.Println("Seeding WU (Cube Rotation)...")
	seedWU(o, subtestMap["WU"])

	// ME (157-176): Memory
	fmt.Println("Seeding ME (Memory)...")
	seedME(o, subtestMap["ME"])

	// Cek total soal yang sudah terisi
	count, _ := o.QueryTable(new(models.ISTQuestion)).Count()
	fmt.Printf("IST full seeder completed successfully. Total questions in database: %d\n", count)
	return nil
}

// SE: Sentence Completion (01-20)
func seedSE(o orm.Ormer, sub *models.ISTSubtest) {
	questions := []struct {
		number int
		prompt string
		opts   []string
		correct string
	}{
		{1, "Pengaruh seseorang terhadap orang lain seharusnya bergantung pada …..", []string{"kekuasaan", "bujukan", "kekayaan", "keberanian", "kewibawaan"}, "E"},
		{2, "Lawannya \"hemat\" ialah ……………", []string{"murah", "kikir", "boros", "bernilai", "kaya"}, "C"},
		{3, "tidak termasuk cuaca", []string{"angin puyuh", "halilintar", "salju", "gempa bumi", "kabut"}, "D"},
		{4, "Lawannya \"setia\" ialah ……………", []string{"cinta", "benci", "persahabatan", "khianat", "permusuhan"}, "D"},
		{5, "Seekor kuda selalu mempunyai ……………", []string{"kandang", "ladam", "pelana", "kuku", "surai"}, "D"},
		{6, "Seorang paman\tlebih tua dari kemenakannya.", []string{"jarang", "biasanya", "selalu", "tidak pernah", "kadang-kadang"}, "B"},
		{7, "Pada jumlah yang sama, nilai kalori yang tertinggi terdapat pada ……………", []string{"ikan", "daging", "lemak", "tahu", "sayuran"}, "C"},
		{8, "Pada suatu pertandingan selalu terdapat ……………", []string{"lawan", "wasit", "penonton", "sorak", "kemenangan"}, "A"},
		{9, "Suatu pernyataan yang belum dipastikan dikatakan sebagai pernyataan yang …..", []string{"paradoks", "tergesa-gesa", "mempunyai arti rangkap", "menyesatkan", "hipotesis"}, "E"},
		{10, "Pada sepatu selalu terdapat ……………", []string{"kulit", "sol", "tali sepatu", "gesper", "lidah"}, "B"},
		{11, "Suatu\ttidak menyangkut persoalan pencegahan kecelakaan.", []string{"lampu lalu lintas", "kacamata pelindung", "kotak PPPK", "tanda peringatan", "palang kereta api"}, "C"},
		{12, "Mata uang logam Rp 50,- tahun 1991, garis tengahnya ialah\tmm.", []string{"17", "29", "25", "20", "15"}, "D"},
		{13, "Seseorang yang bersikap menyangsikan setiap kemajuan ialah seorang yang …..", []string{"demokratis", "radikal", "liberal", "konservatif", "anarkis"}, "D"},
		{14, "Lawannya \"tidak pernah\" ialah ……………", []string{"sering", "kadang-kadang", "jarang", "kerap kali", "selalu"}, "E"},
		{15, "Jarak antara Jakarta – Surabaya kira-kira\tKm", []string{"650", "1000", "800", "600", "950"}, "C"},
		{16, "Untuk dapat membuat nada yang rendah dan mendalam, kita memerlukan banyak ….", []string{"kekuatan", "peranan", "ayunan", "berat", "suara"}, "A"},
		{17, "Ayah\tlebih berpengalaman dari pada anaknya", []string{"selalu", "biasanya", "jauh", "jarang", "pada dasarnya"}, "B"},
		{18, "Diantara kota-kota berikut ini, maka kota\tletaknya paling selatan.", []string{"Jakarta", "Bandung", "Cirebon", "Semarang", "Surabaya"}, "B"},
		{19, "Jika kita mengetahui jumlah presentase nomor-nomor lotere yang tidak menang, maka kita dapat menghitung …..", []string{"jumlah nomor yang menang", "pajak lotere", "kemungkinan menang", "jumlah pengikut", "tinggi keuntungan"}, "C"},
		{20, "Seorang anak yang berumur 10 tahun tingginya rata-rata\tcm", []string{"150", "130", "110", "105", "115"}, "B"},
	}

	for _, q := range questions {
		insertQuestion(o, sub, q.number, q.prompt, q.opts, q.correct, "")
	}
}

// WA: Word Analogies (21-40)
func seedWA(o orm.Ormer, sub *models.ISTSubtest) {
	questions := []struct {
		number int
		prompt string
		opts   []string
		correct string
	}{
		{21, "Carilah kata yang tidak memiliki kesamaan dengan keempat kata lainnya:", []string{"lingkungan", "panah", "elips", "busur", "lengkungan"}, "B"},
		{22, "Carilah kata yang tidak memiliki kesamaan dengan keempat kata lainnya:", []string{"mengetuk", "memaki", "menjahit", "menggergaji", "memukul"}, "B"},
		{23, "Carilah kata yang tidak memiliki kesamaan dengan keempat kata lainnya:", []string{"lebar", "keliling", "luas", "isi", "panjang"}, "D"},
		{24, "Carilah kata yang tidak memiliki kesamaan dengan keempat kata lainnya:", []string{"mengikat", "menyatukan", "melepaskan", "mengaitkan", "melekatkan"}, "C"},
		{25, "Carilah kata yang tidak memiliki kesamaan dengan keempat kata lainnya:", []string{"arah", "timur", "perjalanan", "tujuan", "selatan"}, "C"},
		{26, "Carilah kata yang tidak memiliki kesamaan dengan keempat kata lainnya:", []string{"jarak", "perpisahan", "tugas", "batas", "perceraian"}, "C"},
		{27, "Carilah kata yang tidak memiliki kesamaan dengan keempat kata lainnya:", []string{"saringan", "kelambu", "payung", "tapisan", "jala"}, "C"},
		{28, "Carilah kata yang tidak memiliki kesamaan dengan keempat kata lainnya:", []string{"putih", "pucat", "buram", "kasar", "berkilauan"}, "D"},
		{29, "Carilah kata yang tidak memiliki kesamaan dengan keempat kata lainnya:", []string{"otobis", "pesawat terbang", "sepeda motor", "sepeda", "kapal api"}, "D"},
		{30, "Carilah kata yang tidak memiliki kesamaan dengan keempat kata lainnya:", []string{"biola", "seruling", "klarinet", "terompet", "saxophon"}, "A"},
		{31, "Carilah kata yang tidak memiliki kesamaan dengan keempat kata lainnya:", []string{"bergelombang", "kasar", "berduri", "licin", "lurus"}, "E"},
		{32, "Carilah kata yang tidak memiliki kesamaan dengan keempat kata lainnya:", []string{"jam", "kompas", "penunjuk jalan", "bintang pari", "arah"}, "A"},
		{33, "Carilah kata yang tidak memiliki kesamaan dengan keempat kata lainnya:", []string{"kebijaksanaan", "pendidikan", "perencanaan", "penempatan", "pengerahan"}, "A"},
		{34, "Carilah kata yang tidak memiliki kesamaan dengan keempat kata lainnya:", []string{"bermotor", "berjalan", "berlayar", "bersepeda", "berkuda"}, "B"},
		{35, "Carilah kata yang tidak memiliki kesamaan dengan keempat kata lainnya:", []string{"gambar", "lukisan", "potret", "patung", "ukiran"}, "C"},
		{36, "Carilah kata yang tidak memiliki kesamaan dengan keempat kata lainnya:", []string{"panjang", "lonjong", "runcing", "bulat", "bersudut"}, "A"},
		{37, "Carilah kata yang tidak memiliki kesamaan dengan keempat kata lainnya:", []string{"kunci", "palang pintu", "gerendel", "gunting", "obeng"}, "D"},
		{38, "Carilah kata yang tidak memiliki kesamaan dengan keempat kata lainnya:", []string{"jembatan", "batas", "perkawinan", "pagar", "masyarakat"}, "E"},
		{39, "Carilah kata yang tidak memiliki kesamaan dengan keempat kata lainnya:", []string{"mengetam", "menasehati", "mengasah", "melicinkan", "menggosok"}, "B"},
		{40, "Carilah kata yang tidak memiliki kesamaan dengan keempat kata lainnya:", []string{"batu", "baja", "bulu", "karet", "kayu"}, "C"},
	}

	for _, q := range questions {
		insertQuestion(o, sub, q.number, q.prompt, q.opts, q.correct, "")
	}
}

// AN: Analogies (41-60)
func seedAN(o orm.Ormer, sub *models.ISTSubtest) {
	questions := []struct {
		number int
		prompt string
		opts   []string
		correct string
	}{
		{41, "Menemukan : menghilangkan = Mengingat : ?", []string{"menghapal", "mengenai", "melupakan", "berpikir", "memimpikan"}, "C"},
		{42, "Bunga : jambangan = Burung : ?", []string{"sarang", "langit", "pagar", "pohon", "sangkar"}, "E"},
		{43, "Kereta api : rel = Otobis : ?", []string{"roda", "poros", "ban", "jalan raya", "kecepatan"}, "D"},
		{44, "Perak : emas = Cincin : ?", []string{"arloji", "berlian", "permata", "gelang", "platina"}, "D"},
		{45, "Lingkaran : bola = Bujur sangkar : ?", []string{"bentuk", "gambar", "segi empat", "kubus", "piramida"}, "D"},
		{46, "Saran : kepustakaan = Merundingkan : ?", []string{"menawarkan", "menentukan", "menilai", "menimbang", "merenungkan"}, "A"},
		{47, "Lidah : asam = Hidung : ?", []string{"mencium", "bernapas", "mengecap", "tengik", "asin"}, "D"},
		{48, "Darah : pembuluh = Air : ?", []string{"pintu air", "sungai", "talang", "hujan", "ember"}, "B"},
		{49, "Saraf : penyalur = Pupil : ?", []string{"penyinaran", "mata", "melihat", "cahaya", "pelindung"}, "E"},
		{50, "Pengantar surat : pengantar telegram = Pandai besi : ?", []string{"palu godam", "pedagang besi", "api", "tukang emas", "besi tempa"}, "D"},
		{51, "Buta : warna = Tuli : ?", []string{"pendengaran", "mendengar", "nada", "kata", "telinga"}, "C"},
		{52, "Makanan : bumbu = Ceramah : ?", []string{"penghinaan", "pidato", "kelakar", "kesan", "ayat"}, "C"},
		{53, "Marah : emosi = Duka cita : ?", []string{"suka cita", "sakit hati", "suasana hati", "sedih", "rindu"}, "C"},
		{54, "Mantel : jubah = wool : ?", []string{"bahan sandang", "domba", "sutra", "jas", "tekstil"}, "C"},
		{55, "Ketinggian puncak : tekanan udara = ketinggian nada : ?", []string{"garpu tala", "sopran", "nyanyian", "panjang senar", "suara"}, "D"},
		{56, "Negara : revolusi = Hidup : ?", []string{"biologi", "keturunan", "mutasi", "seleksi", "ilmu hewan"}, "C"},
		{57, "Kekurangan : penemuan = Panas : ?", []string{"haus", "khatulistiwa", "es", "matahari", "dingin"}, "C"},
		{58, "Kayu : diketam = Besi : ?", []string{"dipalu", "digergaji", "dituang", "dikikir", "ditempa"}, "D"},
		{59, "Olahragawan : lembing = Cendekiawan : ?", []string{"perpustakaan", "penelitian", "karya", "studi", "mikroskop"}, "E"},
		{60, "Keledai : kuda pacuan = Pembakaran : ?", []string{"pemadam api", "obor", "letupan", "korek api", "lautan api"}, "E"},
	}

	for _, q := range questions {
		insertQuestion(o, sub, q.number, q.prompt, q.opts, q.correct, "")
	}
}

// GE: General Comprehension (61-76) - Jawaban teks bebas, disimpan sebagai pilihan ganda dengan opsi yang sesuai
func seedGE(o orm.Ormer, sub *models.ISTSubtest) {
	questions := []struct {
		number int
		prompt string
		opts   []string
		correct string
		answerText string // Jawaban yang benar dalam teks
	}{
		{61, "mawar – melati", []string{"bunga", "tanaman", "tumbuhan", "flora", "tumbuhan berbunga"}, "A", "bunga"},
		{62, "mata – telinga", []string{"indera", "panca indera", "organ", "alat indera", "sensor"}, "A", "indera"},
		{63, "gula – intan", []string{"kristal", "zat padat", "bahan", "material", "substansi"}, "A", "kristal"},
		{64, "hujan – salju", []string{"presipitasi", "curah", "air", "fenomena cuaca", "iklim"}, "A", "presipitasi"},
		{65, "pengantar surat – telepon", []string{"komunikasi", "alat komunikasi", "media", "sarana", "perhubungan"}, "A", "komunikasi"},
		{66, "kamera – kacamata", []string{"optik", "lensa", "alat optik", "peralatan", "perangkat"}, "A", "optik"},
		{67, "lambung – usus", []string{"pencernaan", "organ pencernaan", "sistem", "alat", "bagian tubuh"}, "A", "pencernaan"},
		{68, "banyak – sedikit", []string{"kuantitas", "jumlah", "besaran", "ukuran", "volume"}, "A", "kuantitas"},
		{69, "telur – benih", []string{"reproduksi", "perkembangbiakan", "awal kehidupan", "bibit", "asal"}, "A", "reproduksi"},
		{70, "bendera – lencana", []string{"simbol", "lambang", "tanda", "identitas", "atribut"}, "A", "simbol"},
		{71, "rumput – gajah", []string{"makhluk hidup", "organisme", "hewan", "tumbuhan", "biota"}, "A", "makhluk hidup"},
		{72, "ember – kantong", []string{"wadah", "tempat", "kontainer", "alat", "perabot"}, "A", "wadah"},
		{73, "awal – akhir", []string{"waktu", "periode", "rentang", "masa", "tahap"}, "A", "waktu"},
		{74, "kikir – boros", []string{"sifat", "karakter", "perilaku", "kepribadian", "tabiat"}, "A", "sifat"},
		{75, "penawaran – permintaan", []string{"ekonomi", "pasar", "transaksi", "perdagangan", "bisnis"}, "A", "ekonomi"},
		{76, "atas – bawah", []string{"posisi", "arah", "letak", "kedudukan", "tempat"}, "A", "posisi"},
	}

	for _, q := range questions {
		insertQuestion(o, sub, q.number, q.prompt, q.opts, q.correct, "")
	}
}

// RA: Arithmetic (77-96)
func seedRA(o orm.Ormer, sub *models.ISTSubtest) {
	questions := []struct {
		number int
		prompt string
		opts   []string
		correct string
		answerNum string // Jawaban numerik yang benar
	}{
		{77, "Jika seorang anak memiliki 50 rupiah dan memberikan 15 rupiah kepada orang lain, berapa rupiahkah yang masih tinggal padanya?", []string{"35", "40", "45", "30", "25"}, "A", "35"},
		{78, "Berapa km-kah yang dapat ditempuh oleh kereta api dalam waktu 7 jam, jika kecepatannya 40 km/jam?", []string{"280", "270", "290", "300", "260"}, "A", "280"},
		{79, "15 peti buah-buahan beratnya 250 kg dan setiap peti kosong beratnya 3 kg, berapakah berat buah-buahan itu?", []string{"205", "210", "215", "200", "220"}, "A", "205"},
		{80, "Seseorang mempunyai persediaan rumput yang cukup untuk 7 ekor kuda selama 78 hari. Berapa harikah persediaan itu cukup untuk 21 ekor kuda?", []string{"26", "24", "28", "30", "22"}, "A", "26"},
		{81, "3 batang coklat harganya Rp 5,- Berapa batangkah yang dapat kita beli dengan Rp 50,-?", []string{"30", "25", "35", "40", "20"}, "A", "30"},
		{82, "Seseorang dapat berjalan 1,75 m dalam waktu ¼ detik. Berapakah meterkah yang dapat ia tempuh dalam waktu 10 detik?", []string{"70", "65", "75", "80", "60"}, "A", "70"},
		{83, "Jika sebuah batu terletak 15 m di sebelah selatan dari sebatang pohon dan pohon itu berada 30 m di sebelah selatan dari sebuah rumah, berapa meterkah jarak antara batu dan rumah itu?", []string{"45", "40", "50", "35", "55"}, "A", "45"},
		{84, "Jika 4 ½ m bahan sandang harganya Rp 90,- berapakah rupiahkah harganya 2 ½ m?", []string{"50", "45", "55", "60", "40"}, "A", "50"},
		{85, "7 orang dapat menyelesaikan sesuatu pekerjaan dalam 6 hari. Berapa orangkah yang diperlukan untuk menyelesaikan pekerjaan itu dalam setengah hari?", []string{"84", "80", "88", "90", "82"}, "A", "84"},
		{86, "Karena dipanaskan, kawat yang panjangnya 48 cm akan mengembang menjadi 52 cm. setelah pemanasan, berapakah panjangnya kawat yang berukuran 72 cm?", []string{"78", "76", "80", "82", "74"}, "A", "78"},
		{87, "Suatu pabrik dapat menghasilkan 304 batang pensil dalam waktu 8 jam. Berapa batangkah dihasilkan dalam waktu setengah jam?", []string{"19", "18", "20", "21", "17"}, "A", "19"},
		{88, "Untuk suatu campuran diperlukan 2 bagian perak dan 3 bagian timah. Berapa gramkah perak yang diperlukan untuk mendapatkan campuran itu yang beratnya 15 gram?", []string{"6", "5", "7", "8", "4"}, "A", "6"},
		{89, "Untuk setiap Rp 3,- yang dimiliki Sidin, Hamid memiliki Rp 5,- Jika mereka bersama mempunyai Rp 120,- berapa rupiahkah yang dimiliki Hamid?", []string{"75", "70", "80", "85", "65"}, "A", "75"},
		{90, "Mesin A menenun 60 m kain, sedangkan mesin B menenun 40 m. berapa meterkah yang ditenun mesin A, jika mesin B menenun 60 m?", []string{"90", "85", "95", "100", "80"}, "A", "90"},
		{91, "Seseorang membelikan 1/10 dari uangnya untuk perangko dan 4 kali jumlah itu untuk alat tulis. Sisa uangnya masih Rp 60,- Berapa rupiahkah uang semula?", []string{"120", "110", "130", "140", "100"}, "A", "120"},
		{92, "Di dalam dua peti terdapat 43 piring. Di dalam peti yang satu terdapat 9 piring lebih banyak dari pada di dalam peti yang lain. Berapa buah piring terdapat di dalam peti yang lebih kecil?", []string{"17", "16", "18", "19", "15"}, "A", "17"},
		{93, "Suatu lembaran kain yang panjangnya 60 cm harus dibagikan sedemikian rupa sehingga panjangnya satu bagian ialah 2/3 dari bagian yang lain. Berapa panjangnya bagian yang terpendek.", []string{"36", "24", "26", "28", "20"}, "A", "36"},
		{94, "Suatu perusahaan mengekspor ¾ dari hasil produksinya dan menjual 4/5 dari sisa itu dalam negeri. Berapa % kah hasil produksi yang masih tinggal?", []string{"5", "4", "6", "7", "3"}, "A", "5"},
		{95, "Jika suatu botol berisi anggur hanya 7/8 bagian dan harganya ialah Rp 84,- berapakah harga anggur itu jika botol itu hanya terisi ½ penuh?", []string{"48", "46", "50", "52", "44"}, "A", "48"},
		{96, "Di dalam suatu keluarga setiap anak perempuan mempunyai jumlah saudara laki-laki yang sama dengan jumlah saudara perempuan dan setiap anak laki-laki mempunyai dua kali lebih banyak saudara perempuan dari pada saudara laki-laki. Berapa anak laki-lakikah yang terdapat di dalam keluarga tersebut?", []string{"1", "2", "3", "4", "5"}, "A", "1"},
	}

	for _, q := range questions {
		insertQuestion(o, sub, q.number, q.prompt, q.opts, q.correct, "")
	}
}

// ZR: Number Series (97-116)
func seedZR(o orm.Ormer, sub *models.ISTSubtest) {
	questions := []struct {
		number int
		prompt string
		opts   []string
		correct string
		answerNum string
	}{
		{97, "6, 9, 12, 15, 18, 21, 24, ?", []string{"27", "26", "28", "29", "25"}, "A", "27"},
		{98, "15, 16, 18, 19, 21, 22, 24, ?", []string{"25", "24", "26", "27", "23"}, "A", "25"},
		{99, "19, 18, 22, 21, 25, 24, 28, ?", []string{"27", "26", "28", "29", "25"}, "A", "27"},
		{100, "16, 12, 17, 13, 18, 14, 19, ?", []string{"15", "14", "16", "17", "13"}, "A", "15"},
		{101, "2, 4, 8, 10, 20, 22, 44, ?", []string{"46", "45", "47", "48", "44"}, "A", "46"},
		{102, "15, 13, 16, 12, 17, 11, 18, ?", []string{"10", "9", "11", "12", "8"}, "A", "10"},
		{103, "25, 22, 11, 33, 30, 15, 45, ?", []string{"42", "41", "43", "44", "40"}, "A", "42"},
		{104, "49, 51, 54, 27, 9, 11, 14, ?", []string{"7", "6", "8", "9", "5"}, "A", "7"},
		{105, "2, 3, 1, 3, 4, 2, 4, ?", []string{"5", "4", "6", "7", "3"}, "A", "5"},
		{106, "19, 17, 20, 16, 21, 15, 22, ?", []string{"14", "13", "15", "16", "12"}, "A", "14"},
		{107, "94, 92, 46, 44, 22, 20, 10, ?", []string{"8", "7", "9", "10", "6"}, "A", "8"},
		{108, "5, 8, 9, 8, 11, 12, 11, ?", []string{"14", "13", "15", "16", "12"}, "A", "14"},
		{109, "12, 15, 19, 23, 28, 33, 39, ?", []string{"46", "45", "47", "48", "44"}, "A", "45"},
		{110, "7, 5, 10, 7, 21, 17, 68, ?", []string{"65", "64", "66", "67", "63"}, "A", "63"},
		{111, "11, 15, 18, 9, 13, 16, 8, ?", []string{"12", "11", "13", "14", "10"}, "A", "12"},
		{112, "3, 8, 15, 24, 35, 48, 63, ?", []string{"80", "79", "81", "82", "78"}, "A", "80"},
		{113, "4, 5, 7, 4, 8, 13, 7, ?", []string{"14", "21", "23", "24", "20"}, "A", "14"},
		{114, "8, 5, 15, 18, 6, 3, 9, ?", []string{"12", "11", "13", "14", "10"}, "A", "12"},
		{115, "15, 6, 18, 10, 30, 23, 69, ?", []string{"62", "61", "63", "64", "60"}, "A", "63"},
		{116, "5, 35, 28, 4, 11, 77, 70, ?", []string{"10", "9", "11", "12", "8"}, "A", "10"},
	}

	for _, q := range questions {
		insertQuestion(o, sub, q.number, q.prompt, q.opts, q.correct, "")
	}
}

// FA: Shape Assembly (117-136) - Dengan gambar
func seedFA(o orm.Ormer, sub *models.ISTSubtest) {
	// Soal FA memerlukan gambar
	// Gambar ada di folder static/gambar_alat_test/IST/
	// Dapat diakses via URL: /static/gambar_alat_test/IST/{filename}
	imageBasePath := "/static/gambar_alat_test/IST/"
	
	// Kunci jawaban FA berdasarkan screenshot
	faAnswers := map[int]string{
		117: "A", 118: "C", 119: "B", 120: "A", 121: "D", 122: "B", 123: "C", 124: "E",
		125: "E", 126: "D", 127: "E", 128: "B", 129: "D", 130: "C", 131: "B", 132: "A",
		133: "B", 134: "D", 135: "C", 136: "A",
	}
	
	for i := 117; i <= 136; i++ {
		var imageFile string
		if i <= 128 {
			imageFile = "Soal Gambar Kelompok 7 hal 117 - 128.png"
		} else {
			imageFile = "Soal Gmabar Kelompok 7 Hal 129 - 136.png" // Note: typo di nama file asli, tetap gunakan sesuai nama file yang ada
		}
		
		prompt := fmt.Sprintf("Soal nomor %d - Susun potongan-potongan bentuk di bawah ini menjadi bentuk yang sesuai.", i)
		opts := []string{"Bentuk A", "Bentuk B", "Bentuk C", "Bentuk D", "Bentuk E"}
		correct := faAnswers[i]
		if correct == "" {
			correct = "A" // Placeholder, perlu diupdate dari Excel
		}
		
		insertQuestion(o, sub, i, prompt, opts, correct, imageBasePath+imageFile)
	}
}

// WU: Cube Rotation (137-156) - Dengan gambar
func seedWU(o orm.Ormer, sub *models.ISTSubtest) {
	// Gambar ada di folder static/gambar_alat_test/IST/
	// Dapat diakses via URL: /static/gambar_alat_test/IST/{filename}
	imageBasePath := "/static/gambar_alat_test/IST/"
	
	// Kunci jawaban WU berdasarkan screenshot
	wuAnswers := map[int]string{
		137: "A", 138: "C", 139: "D", 140: "E", 141: "A", 142: "C", 143: "D", 144: "C",
		145: "E", 146: "A", 147: "B", 148: "D", 149: "E", 150: "B", 151: "D", 152: "B",
		153: "A", 154: "E", 155: "B", 156: "C",
	}
	
	// Soal 137-141: Soal gambar kelompok 8 hal 137 - 141.png
	// Soal 142-156: Soal gambar kelompok 8 142 - 156.png
	
	for i := 137; i <= 156; i++ {
		var imageFile string
		if i <= 141 {
			imageFile = "Soal gambar kelompok 8 hal 137 - 141.png"
		} else {
			imageFile = "Soal gambar kelompok 8 142 - 156.png"
		}
		
		prompt := fmt.Sprintf("Soal nomor %d - Identifikasi kubus yang sesuai dengan gambar di bawah ini.", i)
		opts := []string{"Kubus A", "Kubus B", "Kubus C", "Kubus D", "Kubus E"}
		correct := wuAnswers[i]
		if correct == "" {
			correct = "A" // Placeholder, perlu diupdate dari Excel
		}
		
		insertQuestion(o, sub, i, prompt, opts, correct, imageBasePath+imageFile)
	}
}

// ME: Memory (157-176) - Soal hafalan kata
func seedME(o orm.Ormer, sub *models.ISTSubtest) {
	// Kata-kata yang perlu dihafal:
	// BUNGA (A): SOKA, LARAT, FLAMBOYAN, YASMIN, DAHLIA
	// PERKAKAS (B): WAJAN, JARUM, KIKIR, CANGKUL, PALU
	// BURUNG (C): ITIK, ELANG, WALET, TERUKUR, NURI
	// KESENIAN (D): QUATET, ARCA, OPERA, UKIRAN, GAMELAN
	// BINATANG (E): RUSA, MUSANG, BERUANG, HARIMAU, ZEBRA
	
	// Mapping huruf pertama ke kategori berdasarkan kunci jawaban screenshot
	// A=bunga, B=perkakas, C=burung, D=kesenian, E=binatang
	letterToAnswer := map[string]string{
		"A": "D", // Soal 157 -> kesenian (QUATET)
		"B": "E", // Soal 158 -> binatang (RUSA)
		"C": "B", // Soal 159 -> perkakas (CANGKUL)
		"D": "A", // Soal 160 -> bunga (DAHLIA)
		"E": "C", // Soal 161 -> burung (ELANG)
		"F": "A", // Soal 162 -> bunga (FLAMBOYAN)
		"G": "D", // Soal 163 -> kesenian (GAMELAN)
		"H": "E", // Soal 164 -> binatang (HARIMAU)
		"I": "C", // Soal 165 -> burung (ITIK)
		"J": "B", // Soal 166 -> perkakas (JARUM)
		"K": "B", // Soal 167 -> perkakas (KIKIR)
		"L": "A", // Soal 168 -> bunga (LARAT)
		"M": "E", // Soal 169 -> binatang (MUSANG)
		"N": "C", // Soal 170 -> burung (NURI)
		"O": "D", // Soal 171 -> kesenian (OPERA)
		"P": "B", // Soal 172 -> perkakas (PALU)
		"Q": "D", // Soal 173 -> kesenian (QUATET)
		"R": "E", // Soal 174 -> binatang (RUSA)
		"S": "A", // Soal 175 -> bunga (SOKA)
		"T": "C", // Soal 176 -> burung (TERUKUR)
	}
	
	// Soal 157-176 sesuai urutan huruf A-U (20 soal)
	letters := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T"}
	
	for i, letter := range letters {
		number := 157 + i
		prompt := fmt.Sprintf("Kata yang mempunyai huruf permulaan – %s – adalah …….", letter)
		opts := []string{"bunga", "perkakas", "burung", "kesenian", "binatang"}
		correct := letterToAnswer[letter]
		if correct == "" {
			correct = "A" // Default fallback
		}
		
		insertQuestion(o, sub, number, prompt, opts, correct, "")
	}
}

// Helper function untuk insert question
func insertQuestion(o orm.Ormer, sub *models.ISTSubtest, number int, prompt string, opts []string, correct string, imageURL string) {
	// Cek apakah soal sudah ada
	var existing models.ISTQuestion
	err := o.QueryTable(new(models.ISTQuestion)).
		Filter("Subtest__Id", sub.Id).
		Filter("Number", number).
		One(&existing)
	if err == nil {
		// Pastikan ada 5 opsi
		for len(opts) < 5 {
			opts = append(opts, "")
		}

		// Selalu sinkronkan soal existing agar kunci & opsi sesuai source of truth terbaru.
		existing.Prompt = prompt
		existing.OptionA = opts[0]
		existing.OptionB = opts[1]
		existing.OptionC = opts[2]
		existing.OptionD = opts[3]
		existing.OptionE = opts[4]
		existing.Correct = strings.ToUpper(correct)
		existing.ImageURL = imageURL
		if _, uerr := o.Update(&existing, "Prompt", "OptionA", "OptionB", "OptionC", "OptionD", "OptionE", "Correct", "ImageURL"); uerr != nil {
			fmt.Printf("Error updating question %s #%d: %v\n", sub.Code, number, uerr)
		}
		return
	}

	// Pastikan ada 5 opsi
	for len(opts) < 5 {
		opts = append(opts, "")
	}

	q := models.ISTQuestion{
		Subtest:  sub,
		Number:   number,
		Prompt:   prompt,
		OptionA:  opts[0],
		OptionB:  opts[1],
		OptionC:  opts[2],
		OptionD:  opts[3],
		OptionE:  opts[4],
		Correct:  strings.ToUpper(correct),
		ImageURL: imageURL,
	}

	if _, err := o.Insert(&q); err != nil {
		fmt.Printf("Error inserting question %s #%d: %v\n", sub.Code, number, err)
	} else {
		fmt.Printf("Inserted question %s #%d\n", sub.Code, number)
	}
}
