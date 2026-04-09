package seeds

import (
	"strings"

	"psikologi_apps/models"

	"github.com/beego/beego/v2/client/orm"
)

type vakSeedItem struct {
	Number    int
	Statement string
	Dimension string
}

var vakQuestions = []vakSeedItem{
	{1, "Saya lebih suka mendengarkan buku dari tape recorder daripada membacanya.", "A"},
	{2, "Ketika saya memasang sesuatu secara bersama-sama, pertama-tama saya lebih membaca petunjuknya.", "V"},
	{3, "Saya lebih suka membaca daripada mendengarkan ceramah.", "V"},
	{4, "Bila saya sedang sendirian, saya selalu bermain musik, bersenandung, atau bernyanyi.", "A"},
	{5, "Saya lebih suka bermain olahraga daripada membaca buku.", "K"},
	{6, "Saya selalu bisa mengatakan atau menunjukkan seperti utara dan selatan tanpa alat di mana saja saya berada.", "V"},
	{7, "Saya suka menulis surat atau sastra atau jurnal.", "V"},
	{8, "Ketika saya berbicara, saya suka mengatakan sesuatu seperti, \"aku dengar ya\".", "A"},
	{9, "Kamarku, meja belajar, mobil, atau rumah selalu teratur.", "V"},
	{10, "Saya suka bekerja dengan tanganku dan membangun atau membuat sesuatu.", "K"},
	{11, "Saya sangat tahu tentang kata-kata atau bunyi yang saya dengarkan.", "A"},
	{12, "Ketika orang lain berbicara, saya selalu membayangkan di pikiran saya tentang apa yang sedang mereka katakan.", "V"},
	{13, "Saya menyukai olahraga dan saya berpendapat bahwa saya orang atlet yang baik.", "K"},
	{14, "Enteng, mudah saya berbicara panjang lebar di telepon dengan teman-teman saya.", "A"},
	{15, "Tanpa musik, hidup ini tidaklah menyenangkan.", "A"},
	{16, "Saya sangat senang ada di organisasi sosial dan selalu memiliki inisiatif pembicaraan dengan banyak orang.", "A"},
	{17, "Ketika saya melihat suatu objek-objek di kertas, saya dapat dengan mudah paham objek-objek itu sama tanpa menggunakan ukuran.", "V"},
	{18, "Saya selalu mengatakan sesuatu seperti, \"ah saya perlu menangani suatu keluhan, atau menemukan satu keluhan\".", "K"},
	{19, "Ketika saya ingat kembali suatu pengalaman, saya sering melihat gambaran tentang pengalaman itu di pikiran saya.", "V"},
	{20, "Ketika saya ingat suatu pengalaman, saya kebanyakan mendengar suara atau ungkapan pada diri saya sendiri tentang pengalaman itu.", "A"},
	{21, "Bila saya ingat suatu pengalaman, saya sering ingat bagaimana saya merasa pada pengalaman itu.", "K"},
	{22, "Saya lebih suka musik daripada lukisan.", "A"},
	{23, "Saya sering mencoret-coret atau menggambar ketika saya sedang menelpon atau dalam suatu pertemuan rapat.", "V"},
	{24, "Saya lebih suka melakukan sesuatu daripada membuat laporan tertulis.", "K"},
	{25, "Saya suka membaca cerita daripada mendengarkan cerita.", "V"},
	{26, "Saya selalu berbicara secara pelan.", "K"},
	{27, "Saya lebih suka berbicara daripada menulis.", "A"},
	{28, "Tulisan tanganku tidak selalu dibutuhkan.", "K"},
	{29, "Saya biasa menggunakan jari untuk menunjuk saat membaca.", "K"},
	{30, "Saya dapat mengalihkan dan menambahkan dengan cepat di kepala saya.", "A"},
	{31, "Saya suka mengeja dan berpendapat saya seorang pembaca yang baik.", "V"},
	{32, "Saya mengeluh sangat bingung bila seseorang mengajak bicara kepadaku ketika sedang menonton TV.", "V"},
	{33, "Saya suka mencatat perintah-perintah yang orang berikan kepadaku.", "V"},
	{34, "Saya dapat mudah mengingat apa yang orang katakan.", "A"},
	{35, "Saya lebih belajar dengan cara melakukan.", "K"},
	{36, "Sangat menyiksa diriku untuk tetap duduk dalam waktu lama.", "K"},
}

func EnsureLearningStyleQuestions() error {
	o := orm.NewOrm()
	for _, item := range vakQuestions {
		var q models.LearningStyleQuestion
		err := o.QueryTable(new(models.LearningStyleQuestion)).
			Filter("Number", item.Number).
			One(&q)
		if err == orm.ErrNoRows {
			newQ := models.LearningStyleQuestion{
				Number:    item.Number,
				Statement: item.Statement,
				Dimension: strings.ToUpper(strings.TrimSpace(item.Dimension)),
			}
			if _, insErr := o.Insert(&newQ); insErr != nil {
				return insErr
			}
			continue
		}
		if err != nil {
			return err
		}
		q.Statement = item.Statement
		q.Dimension = strings.ToUpper(strings.TrimSpace(item.Dimension))
		if _, upErr := o.Update(&q, "Statement", "Dimension"); upErr != nil {
			return upErr
		}
	}
	return nil
}

func LearningStyleInterpretationVisual() string {
	return "Individu dengan tipe ini akan lebih memahami melalui apa yang mereka lihat. Warna, hubungan ruang, potret mental dan gambar menonjol dalam modalitas ini. Adapun beberapa ciri orang dengan tipe belajar visual, yaitu: Rapi, teratur, memperhatikan segala sesuatu dan menjaga penampilan, Berbicara dengan cepat, Perencana dan pengatur jangka panjang yang baik, Pengeja yang baik dan dapat melihat kata-kata yang sebenarnya dalam pikiran mereka, Lebih mengingat apa yang dilihat daripada yang didengar, Mengingat dengan asosiasi visual, Mempunyai masalah untuk mengingat instruksi verbal kecuali jika ditulis dan sering meminta orang lain untuk mengulangi ucapannya, Lebih suka membaca daripada dibacakan dan pembaca yang cepat, Mencoret-coret tanpa arti selama berbicara di telepon atau dalam rapat, Lebih suka melakukan demonstrasi daripada berpidato, Lebih menyukai seni gambar daripada musik, Sering menjawab pertanyaan dengan jawaban singkat ya atau tidak, Mengetahui apa yang harus dikatakan, tetapi tidak pandai memilih kata-kata yang tepat, Biasanya tidak terganggu dengan keributan."
}

func LearningStyleInterpretationAuditory() string {
	return "Individu dengan tipe ini akan lebih memahami sesuatu melalui apa yang mereka dengar. Modalitas ini mengakses segala jenis bunyi dan kata. Musik, irama, dialog internal dan suara menonjol pada tipe auditori. Seseorang yang sangat auditori memiliki ciri-ciri sebagai berikut: Suka berbicara kepada diri sendiri saat bekerja, Perhatiannya mudah terpecah dan mudah terganggu oleh keributan, Menggerakkan bibir mereka dan mengucapkan tulisan di buku ketika membaca, Senang membaca dengan keras dan mendengarkan, Dapat mengulangi kembali dan menirukan nada, perubahan dan warna suara, Merasa kesulitan untuk menulis dan lebih suka mengucapkan secara lisan, Berbicara dalam irama yang terpola, Lebih suka musik daripada seni gambar, Belajar dengan mendengarkan dan mengingat apa yang didiskusikan daripada yang dilihat, Suka berbicara, suka berdiskusi dan menjelaskan sesuatu dengan panjang lebar, Lebih suka gurauan lisan daripada membaca komik, Mempunyai masalah dengan pekerjaan-pekerjaan yang melibatkan visualisasi, seperti memotong bagian-bagian hingga sesuai satu sama lain, Lebih pandai mengeja dengan keras daripada menuliskannya, Biasanya pembicara yang fasih."
}

func LearningStyleInterpretationKinesthetic() string {
	return "Individu dengan tipe ini belajar melalui gerak, emosi dan sentuhan. Modalitas ini mengakses pada gerakan, koordinasi, irama, tanggapan emosional, dan kenyamanan fisik. Ciri-ciri orang dengan tipe belajar kinestetik yaitu: Berbicara dengan perlahan, Menyentuh orang untuk mendapatkan perhatian mereka saat berbicara, Berdiri berdekatan saat berbicara dengan orang, Selalu berorientasi pada fisik dan banyak bergerak, Belajar melalui memanipulasi dan praktik, Menghafal dengan cara berjalan dan melihat, Menggunakan jari sebagai penunjuk ketika membaca, Banyak menggunakan isyarat tubuh, Tidak dapat diam untuk waktu yang lama, Tidak dapat mengingat geografis, kecuali jika mereka memang telah pernah berada di tempat itu, Menyukai permainan yang menyibukkan, Mencerminkan aksi dengan gerakan tubuh saat membaca, suka mengetuk-ngetuk pena, jari, atau kaki saat mendengarkan, Ingin melakukan segala sesuatu, Kemungkinan tulisannya kurang bagus."
}
