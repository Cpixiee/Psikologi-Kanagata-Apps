package seeds

import (
	"fmt"

	"github.com/beego/beego/v2/client/orm"
)

// RMIB category order baku (rotasi per kelompok dimulai dari index = group-1).
// Order index 0..11: OUT, MEC, COMP, SCI, PERS, AEST, MUS, LIT, SOC, CLER, PRAC, MED.
var rmibCategoryOrder = []string{
	"OUT", "MEC", "COMP", "SCI", "PERS", "AEST", "MUS", "LIT", "SOC", "CLER", "PRAC", "MED",
}

type rmibGroupPria struct {
	number      int
	title       string
	description string
	items       []string // 12 item, urut sesuai naskah RMIB pria
}

// rmibPriaGroups berisi 8 kelompok x 12 aktivitas (96 item) sesuai naskah RMIB Pria
// (proyek Wisma Kerja Nyata). Mapping kategori dihitung lewat rotasi:
//
//	category(group g, item i) = rmibCategoryOrder[(g - 1 + i - 1) mod 12]
var rmibPriaGroups = []rmibGroupPria{
	{
		number: 1,
		title:  "Persiapan Perjalanan",
		description: "Tim mengadakan persiapan perjalanan. Rapat-rapat perlu diadakan. Dana dari sponsor, " +
			"kendaraan, peralatan, dan sebagainya perlu. Urutkan tugas yang paling Anda senangi.",
		items: []string{
			"Menyetir mobil antar-jemput para anggota yang sibuk dengan urusan, dari satu tempat ke tempat lain.",
			"Mempelajari cara memperbaiki kerusakan-kerusakan ringan pada mesin mobil yang siap dipakai tim.",
			"Merencanakan anggaran belanja proyek perjalanan yang akan diajukan kepada sponsor.",
			"Membaca laporan-laporan penelitian KLH untuk memilih proyek KLH yang tepat.",
			"Menghadap sponsor untuk tawar-menawar dana di perusahaan \"Supra Motor\".",
			"Merancang simbol dan logo untuk peralatan tim WKN.",
			"Memilih berbagai khazanah lagu-lagu yang tepat bagi kampanye KLH.",
			"Menyiapkan bacaan populer dan membuat kliping dari koran/majalah.",
			"Menjadi penghubung dengan kelompok WKN lain (karena punya banyak teman/kenalan).",
			"Mencatat hasil, menyusun, dan menyimpan arsip.",
			"Menyiapkan peralatan makan, bahan, makan/perbekalan.",
			"Menyiapkan peralatan kedokteran, P3K, dan obat-obatan.",
		},
	},
	{
		number: 2,
		title:  "Bazar Malam",
		description: "Anggota uang melakukan tawar-menawar dana mengabarkan, bahwa keduabelasan Anda diminta " +
			"membantu pelaksanaan \"bazar malam\" untuk menjual barang-barang simpanan sponsor. " +
			"Keduabelasan Anda akan mendapat 20% hasil penjualan sebagai tambahan bekal. Pilih sendiri tugas yang Anda senangi.",
		items: []string{
			"Menyervis mesin diesel dan menyolder komponen panel-panel pengatur lampu iklan.",
			"Menghitung pengeluaran dan pemasukan bazar (dan 20% hasil penjualannya).",
			"Meneliti ulat dan kutu yang telah menyerang gudang barang-barang yang akan \"dibazarkan\".",
			"Menggunakan pengeras suara untuk menarik pengunjung agar membeli.",
			"Merancang dan menggambarkan papan reklame bazar.",
			"Menggubah lagu-lagu yang tepat dan memikat untuk iklan bazar yang disiarkan melalui radio.",
			"Mengarang sajak-sajak jenaka untuk dibacakan guna memeriahkan acara bazar.",
			"Membantu para pengunjung mengisi daftar pesanan barang yang perlu diantar ke rumah.",
			"Mengetik undangan khusus untuk para pejabat/jutawan.",
			"Memasang tenda, spanduk rumbai-rumbai dan bendera.",
			"Membantu dokter melayani pengobatan cuma-cuma bagi pengunjung.",
			"Menjaga keamanan di halaman gedung, siap dengan \"halo-halo\" (walky-talky).",
		},
	},
	{
		number: 3,
		title:  "Pembenahan Wisma",
		description: "Persiapan berjalan mulus. Perjalanan juga menyenangkan. Di Kecamatan Depok, Pak Camat minta " +
			"agar tim WKN membenahi \"Wisma Padepokan KLH\" tempat tim Anda mengadakan Pameran dan Kampanye KLH. " +
			"Urutkan pilihan tugas Anda.",
		items: []string{
			"Melakukan perhitungan biaya pembenahan wisma.",
			"Meneliti apakah ular-ular yang berkeliaran di wisma berbisa.",
			"Menghadap Pak Camat untuk meyakinkan bahwa proyek pembenahan wisma tersebut, meskipun mahal, masih seimbang dari segi manfaat.",
			"Menggambarkan sketsa tokoh-tokoh KLH.",
			"Membuat aransemen lagu \"Lestarikan Lingkunganku\" yang akan dimainkan Band Musik Remaja Depok.",
			"Menulis puisi yang mendukung KLH untuk lomba deklamasi anak-anak/remaja Depok.",
			"Menyambut penduduk yang datang di wisma untuk berdiskusi mengenai pencemaran lingkungan.",
			"Menyimpan arsip surat-surat dan rekening-rekening.",
			"Mengecat tembok wisma.",
			"Mengobati penduduk Depok yang menderita gatal-gatal.",
			"Mengatur letak batu-batuan dalam kolam di halaman wisma.",
			"Memperbaiki pompa air yang macet karena lama tidak dipakai.",
		},
	},
	{
		number: 4,
		title:  "Kampanye Lingkungan",
		description: "Kampanye di wisma direncanakan terdiri atas beberapa acara, antara lain: penghijauan, " +
			"kependidikan, dan masalah polusi. Urutkan tugas-tugas yang Anda pilih.",
		items: []string{
			"Memeriksa unsur-unsur kimiawi air limbah membusuk yang diduga tercemar.",
			"Menjadi anggota utusan yang menghimbau direksi pabrik agar memperbaiki pengolahan limbah.",
			"Merancang/menggambar poster dan spanduk kampanye menggunakan cat minyak.",
			"Menggubah lagu sederhana kampanye penghijauan.",
			"Menyiapkan kata-kata brosur dan selebaran Kampanye Penghijauan dan Keluarga Berencana.",
			"Memberi penjelasan kepada para pengunjung pameran.",
			"Mengetik naskah dan memperbanyak naskah.",
			"Mengatur meja, kursi, dan lemari pameran.",
			"Menjadi \"mantri kesehatan\" di Puskesmas setempat.",
			"Menanamkan pohon-pohon lindung di jalur hijau.",
			"Merakit dan memasang pompa air bersih untuk penduduk.",
			"Membantu penduduk menghitung ganti rugi akibat pencemaran.",
		},
	},
	{
		number:      5,
		title:       "Lomba Remaja",
		description: "Bagi para remaja dan anak-anak, KLH diperkenalkan dengan mengadakan berbagai lomba. Urutkan pilihan Anda.",
		items: []string{
			"Juri lomba pidato mengenai KLH.",
			"Juri lomba seni lukis dan seni patung.",
			"Juri lomba seni memainkan instrumen musik daerah.",
			"Juri lomba deklamasi puisi/membaca prosa KLH.",
			"Juri \"10 Remaja Pria yang Pandai Bergaul\".",
			"Juri lomba mengetik cepat dan rapih.",
			"Juri lomba memasak nasi goreng dan mie instan.",
			"Juri lomba Palang Merah Remaja dan dokter kecil.",
			"Juri lomba voli dan sepak bola.",
			"Juri lomba reparasi sepeda.",
			"Juri lomba matematika SD.",
			"Juri lomba penelitian populer.",
		},
	},
	{
		number: 6,
		title:  "Menggantikan Guru",
		description: "Guru SD perlu mendapatkan pengetahuan lebih mendalam mengenai KLH. Mereka mendapat penataran, " +
			"dan Anda perlu menggantikan tugas mereka. Urutkan pilihan Anda.",
		items: []string{
			"Mengajar seni lukis dan seni patung.",
			"Mengajar memainkan musik dan menyanyi.",
			"Mengajar bahasa dan mengarang.",
			"Mengajar mata pelajaran agama.",
			"Mengajar mengetik surat.",
			"Mengajar prakarya.",
			"Mengajar teknik merawat orang sakit.",
			"Mengajar olahraga dan sepak bola.",
			"Mengajar penggunaan alat-alat pertukangan sederhana.",
			"Mengajar berhitung.",
			"Mengajar ilmu hewan, ilmu tumbuh-tumbuhan.",
			"Menjelaskan seluk beluk koperasi (IPS).",
		},
	},
	{
		number: 7,
		title:  "Agen Rahasia",
		description: "Tim Anda diminta menjadi agen rahasia untuk menyelidiki satu keluarga pedagang kaya raya yang diduga " +
			"terlibat dalam masalah narkotika. Keluarga ini akan mengadakan pesta besar-besaran di salah satu Pulau Seribu. " +
			"Setiap anggota tim menyamar sebagai petugas. Urutkan pilihan tugas Anda.",
		items: []string{
			"Menjadi gitaris/pianis \"Band Musik\".",
			"Menjadi \"asisten pengarang\" yang membuatkan teks undangan berbentuk puisi yang aduhai.",
			"Menyambut tamu dan menunjukkan tempat penginapan masing-masing.",
			"Menjadi sekretaris, yang antara lain mengetik nama tamu yang telah hadir.",
			"Menjadi asisten utama koki untuk khusus tamu VIP.",
			"Menjadi perawat di Pos Kesehatan, terutama untuk penyakit gawat darurat (jantung, kecelakaan, dsb.).",
			"Petugas pengatur lalu lintas mobil-mobil jemputan.",
			"Menjaga diesel pembangkit listrik yang sering mogok.",
			"Juru bayar rekening pesta.",
			"Bekerja di laboratorium bahan makanan.",
			"Menjadi pengarah acara sambil menghimbau dermawan untuk membantu proyek KLH.",
			"Membuat patung besar \"SELAMAT DATANG\" dari tanah liat.",
		},
	},
	{
		number: 8,
		title:  "Bantuan Bencana",
		description: "Di suatu daerah, terjadi musibah gempa yang diikuti dengan tanah longsor yang melanda daerah pemukiman. " +
			"Tim Anda diminta membantu. Urutkan tugas pilihan Anda.",
		items: []string{
			"Menulis karangan agar musibah ini mendapat perhatian masyarakat.",
			"Menenangkan mereka yang terkena musibah.",
			"Mengetik surat jalan bagi mereka yang ingin mengungsi ke luar daerah.",
			"Memasang tenda untuk penampungan mereka yang kehilangan tempat tinggal.",
			"Membantu merawat mereka yang luka parah.",
			"Membantu penggalian reruntuhan di lokasi/tempat musibah.",
			"Memasang kabel-kabel lampu darurat agar penggalian dapat berlangsung siang-malam.",
			"Menjadi kasir yang menyalurkan uang bantuan.",
			"Membawa berbagai peralatan untuk menyelidiki tempat-tempat yang rawan longsor.",
			"Menghadap para multi jutawan untuk menghimbau dana bantuan.",
			"Membuat gambar/patung untuk dijual oleh pencari dana.",
			"Ikut sebagai pemain band untuk mencari dana kemanusiaan.",
		},
	},
}

// SeedRMIBPria mengisi/upsert 96 item RMIB pria. Idempoten: aman dipanggil
// berulang (UPSERT by gender_version, group_number, item_order).
func SeedRMIBPria() error {
	o := orm.NewOrm()

	for _, g := range rmibPriaGroups {
		if len(g.items) != 12 {
			return fmt.Errorf("RMIB pria group %d harus berisi 12 item, ditemukan %d", g.number, len(g.items))
		}
		for i, text := range g.items {
			itemOrder := i + 1
			catIdx := (g.number - 1 + i) % 12
			category := rmibCategoryOrder[catIdx]

			_, err := o.Raw(`
				INSERT INTO rmib_questions (gender_version, group_number, group_title, group_description, item_order, question_text, category_code)
				VALUES (?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (gender_version, group_number, item_order)
				DO UPDATE SET
					group_title = EXCLUDED.group_title,
					group_description = EXCLUDED.group_description,
					question_text = EXCLUDED.question_text,
					category_code = EXCLUDED.category_code
			`, "pria", g.number, g.title, g.description, itemOrder, text, category).Exec()
			if err != nil {
				return fmt.Errorf("upsert RMIB pria group %d item %d: %w", g.number, itemOrder, err)
			}
		}
	}
	return nil
}
