package seeds

import (
	"fmt"

	"github.com/beego/beego/v2/client/orm"
)

// rmibWanitaGroups berisi 8 kelompok x 12 aktivitas (96 item) sesuai naskah RMIB Wanita
// (proyek Wisma Kerja Nyata - WKN versi mahasiswa wanita).
//
// Mapping kategori dihitung lewat rotasi yang sama dengan versi pria:
//
//	category(group g, item i) = rmibCategoryOrder[(g - 1 + i - 1) mod 12]
//
// Sehingga skoring dan tafsir kategori (Outdoor, Mechanical, Computational, Scientific,
// Personal Contact, Aesthetic, Musical, Literary, Social Service, Clerical, Practical,
// Medical) konsisten antara versi pria dan wanita.
var rmibWanitaGroups = []rmibGroupPria{
	{
		number: 1,
		title:  "Persiapan Perjalanan",
		description: "Tim mengadakan persiapan perjalanan. Rapat-rapat perlu diadakan. Dana dari sponsor, " +
			"kendaraan, peralatan, dan sebagainya perlu diurus. Urutkan tugas yang paling Anda senangi " +
			"dengan menuliskan nomor pilihan pada kolom di bawah \"No\".",
		items: []string{
			"Siap sebagai sopir cadangan selama perjalanan.",
			"Mempelajari cara mengganti ban/memeriksa oli mobil.",
			"Merencanakan anggaran belanja proyek perjalanan yang akan diajukan kepada sponsor.",
			"Membaca setumpuk laporan penelitian KLH untuk memilih proyek KLH yang tepat.",
			"Menjadi anggota delegasi yang menghadap Ibu Direksi (sponsor) untuk tawar-menawar dana.",
			"Membuat gambar, simbol, dan logo untuk tim WKN.",
			"Memilih aransemen lagu yang tepat untuk kampanye Kementerian Lingkungan Hidup.",
			"Menyiapkan bacaan-bacaan populer KLH untuk tingkat remaja.",
			"Menjadi penghubung dengan kelompok-kelompok WKN lain (karena punya banyak tempat/kenalan).",
			"Mencatat rencana-rencana, menyusun, dan menyimpan arsip.",
			"Menyiapkan peralatan makan, bahan makanan/perbekalan.",
			"Menyiapkan peralatan kesehatan dan obat-obatan bagi hewan dan manusia.",
		},
	},
	{
		number: 2,
		title:  "Bazar Malam",
		description: "Anggota yang melakukan tawar-menawar dana mengabarkan bahwa kedua-belasan Anda " +
			"diminta membantu pelaksanaan \"bazar malam\" untuk menjual barang-barang simpanan sponsor. " +
			"Kedua-belasan Anda akan mendapat 20% hasil penjualan sebagai tambahan bekal. Pilih sendiri tugas yang Anda senangi.",
		items: []string{
			"Meminyaki mesin jahit yang dipakai untuk menjahit pakaian yang dibazarkan.",
			"Menghitung pengeluaran dan pemasukan bazar (dan 20% hasil penjualannya).",
			"Bekerja di laboratorium meneliti kemurnian insektisida yang akan \"dibazarkan\".",
			"Menggunakan pengeras suara untuk menarik pengunjung agar membeli.",
			"Melukis papan reklame bazar menggunakan kuas dan cat.",
			"Mengarang lagu-lagu KLH sederhana, yang mudah ditiru anak-anak kecil untuk disiarkan melalui radio.",
			"Memilih sajak-sajak jenaka untuk memeriahkan acara bazar.",
			"Membantu para pengunjung mengisi daftar pesanan barang yang perlu diantar ke rumah.",
			"Mengetik undangan khusus untuk para pejabat/jutawan.",
			"Memasak dan mengatur meja makanan di kafetaria.",
			"Menjadi asisten/pembantu dokter dalam rangka pengobatan cuma-cuma bagi pengunjung.",
			"Menjaga keamanan anak-anak yang sedang bermain di halaman gedung.",
		},
	},
	{
		number: 3,
		title:  "Pembenahan Wisma",
		description: "Persiapan berjalan mulus. Perjalanan juga menyenangkan. Di Kecamatan Depok, Pak Camat minta " +
			"agar tim WKN membenahi \"Wisma Padepokan KLH\" tempat tim Anda menginap dan mengadakan Pameran serta Kampanye KLH. " +
			"Urutkan pilihan tugas Anda.",
		items: []string{
			"Mengukur panjang dan lebarnya kain gorden dan permadani yang dibutuhkan di Wisma.",
			"Memeriksa di laboratorium air sumur Wisma yang berbau busuk yang diduga terkena pencemaran.",
			"Menghimbau Ibu Camat dan ibu-ibu PKK untuk membantu pembenahan Wisma.",
			"Menyiapkan gambar sketsa perubahan dekorasi tata ruang.",
			"Memilih aransemen lagu \"Lestarikan Lingkunganku\" yang akan dimainkan Bank Musik Remaja Depok.",
			"Menulis puisi yang mendukung KLH untuk lomba deklamasi anak-anak/remaja Depok.",
			"Menerima tamu-tamu siswa setempat yang meminta informasi mengenai KLH.",
			"Menyimpan arsip rekening-rekening tagihan.",
			"Membersihkan wisma dan perkakas rumah tangga.",
			"Mengobati penduduk Depok yang menderita gatal-gatal di tenggorokan.",
			"Mengatur tanaman di kolam di halaman wisma.",
			"Memasang kembali kran air yang dicopot karena lama tidak dipakai.",
		},
	},
	{
		number:      4,
		title:       "Kampanye Lingkungan",
		description: "Kampanye di Wisma direncanakan terdiri atas beberapa acara, antara lain: penghijauan, kependudukan, dan masalah polusi. Urutkan tugas-tugas yang Anda pilih.",
		items: []string{
			"Memeriksa penyakit hewan yang minum air sungai yang tercampur limbah industri.",
			"Menjadi anggota utusan yang menghimbau direksi pabrik agar memperbaiki pengolahan limbah.",
			"Merancang gambar, poster, dan spanduk kampanye.",
			"Mengarang lagu sederhana kampanye penghijauan.",
			"Menyiapkan kata-kata penerangan melalui radio.",
			"Memberi penjelasan kepada para siswa SMA yang mengunjungi pameran.",
			"Mengetik naskah dan membantu tata usaha.",
			"Mengatur meja, kursi, dan lemari pameran.",
			"Menjadi anggota \"tim kesehatan\" yang mengobati penduduk yang sakit.",
			"Menanamkan bibit semak/perdu di jalur hijau.",
			"Merakit saringan air bersih untuk penduduk.",
			"Membantu menghitung besarnya ganti bagi penduduk yang terkena pencemaran.",
		},
	},
	{
		number:      5,
		title:       "Lomba Remaja",
		description: "Bagi para remaja dan anak-anak, KLH diperkenalkan dengan mengadakan berbagai lomba. Urutkan pilihan Anda.",
		items: []string{
			"Juri lomba pidato mengenai KLH.",
			"Juri lomba seni merangkai bunga.",
			"Juri lomba menyanyi lagu daerah.",
			"Juri lomba merangkai kata-kata menjadi cerita KLH.",
			"Juri lomba \"Remaja putri ngetop dalam bergaul\".",
			"Juri lomba mengetik cepat dan rapih.",
			"Juri lomba memasak nasi goreng paling istimewa.",
			"Juri lomba PPPK dan Palang Merah Remaja.",
			"Juri lomba voli.",
			"Juri lomba menambal ban sepeda.",
			"Juri lomba matematika murid SD.",
			"Juri lomba proyek observasi IPA.",
		},
	},
	{
		number: 6,
		title:  "Menggantikan Guru",
		description: "Guru SD mendapat pengetahuan lebih mendalam mengenai KLH. Mereka mendapat penataran, " +
			"dan Anda perlu menggantikan tugas mereka. Urutkan pilihan Anda.",
		items: []string{
			"Mengajar menggambar dengan cat air.",
			"Mengajar memainkan angklung/kulintang.",
			"Mengajar mengarang mengenai KLH.",
			"Menggantikan tugas guru agama.",
			"Mengajar anak-anak menulis dengan mesin ketik.",
			"Mengajar memasak lauk pauk sederhana.",
			"Mengajar cara membersihkan luka yang infeksi.",
			"Mengajar olahraga.",
			"Mengajar murid-murid merakit serutan pensil.",
			"Mengajar berhitung praktis.",
			"Mengajar ilmu hewan dan tumbuh-tumbuhan.",
			"Mengajar dasar-dasar koperasi sekolah.",
		},
	},
	{
		number: 7,
		title:  "Agen Rahasia",
		description: "Tim Anda diminta menjadi agen rahasia untuk menyelidiki satu keluarga pedagang kaya raya " +
			"yang diduga terlibat dalam masalah narkotika. Keluarga ini akan mengadakan pesta besar-besaran " +
			"di salah satu pulau di Kepulauan Seribu. Setiap anggota tim menyamar sebagai petugas. Urutkan pilihan tugas Anda.",
		items: []string{
			"Menjadi pianis/gitaris \"Band Musik\".",
			"Menjadi \"asisten pengarang\" yang membuatkan teks undangan berbentuk puisi yang aduhai.",
			"Menyambut tamu dan menyilahkan mengisi formulir kedatangan dan buku tamu.",
			"Menjadi sekretaris, yang mencatat dan menyimpan formulir kedatangan untuk keperluan penyidikan.",
			"Menyiapkan hidangan dan mengedarkan makanan diantara bagi para tamu.",
			"Menjadi perawat yang siap di Pos Kesehatan Darurat bagi para tamu.",
			"Menjadi petugas (Polwan) yang mengatur lalu lintas mobil tamu-tamu.",
			"Siap menyekrup di tempat yang tersedia papan nama tamu-tamu yang telah datang.",
			"Menjadi kasir yang membayar semua rekening pesta.",
			"Bekerja di laboratorium penyelidikan sidik jari para tamu.",
			"Menjadi pengarah acara sambil menghimbau dermawan untuk membantu proyek KLH.",
			"Bersama-sama membuat patung dari bongkah es.",
		},
	},
	{
		number: 8,
		title:  "Bantuan Bencana",
		description: "Di suatu daerah, terjadi musibah gempa yang diikuti dengan tanah longsor yang melanda " +
			"daerah pemukiman. Tim Anda diminta membantu. Urutkan tugas pilihan Anda.",
		items: []string{
			"Menulis \"Pikiran Pembaca\" agar musibah ini mendapat perhatian masyarakat.",
			"Menenangkan mereka yang kebingungan karena terkena musibah.",
			"Mengetik daftar nama korban yang memerlukan bantuan.",
			"Mengatur tempat tidur darurat untuk menampung mereka yang kehilangan tempat tinggal.",
			"Merawat mereka yang luka parah.",
			"Siap berada di sekitar reruntuhan lokasi/tempat musibah untuk membantu apa pun juga.",
			"Merakit lampu darurat agar penggalian dapat berlangsung siang malam.",
			"Menjadi kasir yang membagikan uang bantuan kepada yang berhak.",
			"Membaca laporan-laporan mengenai kerawanan di daerah-daerah sekitar musibah untuk membantu perencanaan tindak lanjut.",
			"Menghadap para jutawan untuk membahas pemberian bantuan.",
			"Membantu membuat patung peringatan terhadap terjadinya musibah.",
			"Menjadi anggota paduan suara untuk mencari dana kemanusiaan.",
		},
	},
}

// SeedRMIBWanita mengisi/upsert 96 item RMIB wanita. Idempoten: aman dipanggil
// berulang (UPSERT by gender_version, group_number, item_order).
func SeedRMIBWanita() error {
	o := orm.NewOrm()

	for _, g := range rmibWanitaGroups {
		if len(g.items) != 12 {
			return fmt.Errorf("RMIB wanita group %d harus berisi 12 item, ditemukan %d", g.number, len(g.items))
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
			`, "wanita", g.number, g.title, g.description, itemOrder, text, category).Exec()
			if err != nil {
				return fmt.Errorf("upsert RMIB wanita group %d item %d: %w", g.number, itemOrder, err)
			}
		}
	}
	return nil
}
