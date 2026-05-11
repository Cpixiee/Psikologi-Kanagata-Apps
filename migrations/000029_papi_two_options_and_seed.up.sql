-- Ubah skema PAPI menjadi paired comparison (hanya OptionA & OptionB)
-- 1) Hapus kolom yang tidak terpakai (option_c, option_d, category_c, category_d, question_text)
-- 2) Sesuaikan CHECK constraint selected_option pada papi_answers menjadi A/B saja
-- 3) Bersihkan data lama (jika ada) dan seed 90 soal PAPI dengan keying resmi
--    (10 Peran: G L I T V S R D C E + 10 Kebutuhan: N A P X B O Z K F W)
-- 4) Perlonggar size kolom kategori (cukup 2 char) dan ganti CHECK selected_option
--    serta kembalikan size category_a/b dari 8 ke 2 untuk konsistensi.

ALTER TABLE papi_questions DROP COLUMN IF EXISTS option_c;
ALTER TABLE papi_questions DROP COLUMN IF EXISTS option_d;
ALTER TABLE papi_questions DROP COLUMN IF EXISTS category_c;
ALTER TABLE papi_questions DROP COLUMN IF EXISTS category_d;
ALTER TABLE papi_questions DROP COLUMN IF EXISTS question_text;

-- Hapus jawaban lama yang mungkin berisi C/D agar konsisten dengan format baru
DELETE FROM papi_answers WHERE selected_option NOT IN ('A', 'B');

-- Ganti CHECK constraint selected_option (Postgres tidak punya IF EXISTS untuk constraint
-- name yang tidak diketahui, jadi kita drop berdasarkan introspeksi pg_constraint).
DO $$
DECLARE
    cname text;
BEGIN
    SELECT conname INTO cname
    FROM pg_constraint
    WHERE conrelid = 'papi_answers'::regclass
      AND contype = 'c'
      AND pg_get_constraintdef(oid) ILIKE '%selected_option%';
    IF cname IS NOT NULL THEN
        EXECUTE format('ALTER TABLE papi_answers DROP CONSTRAINT %I', cname);
    END IF;
END$$;

ALTER TABLE papi_answers
    ADD CONSTRAINT papi_answers_selected_option_check
    CHECK (selected_option IN ('A', 'B'));

-- Bersihkan data soal lama dan seed ulang dengan keying PAPI resmi
TRUNCATE TABLE papi_questions RESTART IDENTITY CASCADE;

-- Format INSERT: (item_number, option_a, option_b, category_a, category_b)
-- category_a = kode untuk pernyataan ATAS, category_b = kode untuk pernyataan BAWAH
INSERT INTO papi_questions (item_number, option_a, option_b, category_a, category_b) VALUES
(1,  'Saya seorang pekerja keras', 'Saya bukan seorang pemurung', 'G', 'E'),
(2,  'Saya suka bekerja lebih baik dari yang lain', 'Saya senang menekuni pekerjaan yang saya lakukan sampai selesai', 'A', 'N'),
(3,  'Saya suka memberi petunjuk kepada orang bagaimana melakukan sesuatu', 'Saya ingin melakukan sesuatu sebaik mungkin', 'P', 'A'),
(4,  'Saya suka melakukan hal-hal yang lucu', 'Saya senang memberi tahu orang apa yang harus dikerjakannya', 'X', 'P'),
(5,  'Saya suka bergabung dengan kelompok', 'Saya senang diperhatikan oleh kelompok', 'B', 'X'),
(6,  'Saya suka membina suatu hubungan persahabatan antar pribadi', 'Saya suka berteman dengan suatu kelompok', 'O', 'B'),
(7,  'Cepat berubah jika saya rasa hal itu diperlukan', 'Saya berusaha membina hubungan yang akrab dengan teman saya', 'Z', 'O'),
(8,  'Saya suka membalas jika saya disakiti', 'Saya suka melakukan hal-hal yang baru dan berbeda', 'K', 'Z'),
(9,  'Saya ingin atasan saya menyukai saya', 'Saya suka memberi tahu orang jika mereka salah', 'F', 'K'),
(10, 'Saya suka mengikuti petunjuk-petunjuk yang diberikan kepada saya', 'Saya suka mendukung pendapat atasan saya', 'W', 'F'),
(11, 'Saya berusaha sangat keras', 'Saya seorang teratur, saya menaruh semua barang pada tempatnya', 'G', 'C'),
(12, 'Saya dapat membuat orang mau bekerja keras', 'Saya tidak mudah marah', 'L', 'C'),
(13, 'Saya suka memberi tahu kelompok apa yang harus dikerjakan', 'Saya selalu menekuni suatu pekerjaan sampai selesai', 'P', 'N'),
(14, 'Saya ingin tampil menarik dan menakjubkan', 'Saya ingin menjadi orang yang berhasil', 'X', 'A'),
(15, 'Saya ingin sesuai dan diterima dalam kelompok', 'Saya suka membantu orang lain mengambil sikap', 'B', 'P'),
(16, 'Saya cemas jika seseorang tidak menyukai saya', 'Saya suka orang memperhatikan saya', 'O', 'X'),
(17, 'Saya suka mencoba hal-hal baru', 'Saya lebih suka bekerja bersama orang lain daripada sendiri', 'Z', 'B'),
(18, 'Saya kadang-kadang menyalahkan orang lain jika ada terjadi kesalahan', 'Saya merasa terganggu jika ada yang tidak menyukai saya', 'K', 'O'),
(19, 'Saya suka mendukung pendapat atasan saya', 'Saya suka mencoba pekerjaan-pekerjaan yang baru dan berbeda', 'F', 'Z'),
(20, 'Saya menyukai petunjuk terperinci dalam menyelesaikan permasalahan', 'Saya suka memberi tahu bila orang membuat saya kesal', 'W', 'K'),
(21, 'Saya selalu berusaha keras', 'Saya suka melaksanakan setiap langkah dengan hati-hati', 'G', 'D'),
(22, 'Saya seorang pemimpin yang baik', 'Saya dapat mengorganisir suatu pekerjaan dengan baik', 'L', 'C'),
(23, 'Saya mudah tersinggung', 'Saya lambat dalam membuat keputusan', 'I', 'E'),
(24, 'Dalam suatu kelompok saya lebih suka diam', 'Saya suka mengerjakan beberapa pekerjaan sekaligus', 'X', 'N'),
(25, 'Saya sangat suka bila saya diundang', 'Saya ingin lebih baik dari yang lain dalam mengerjakan sesuatu', 'B', 'A'),
(26, 'Saya suka membina hubungan yang akrab dengan teman-teman saya', 'Saya suka menasehati orang lain', 'O', 'P'),
(27, 'Saya suka melakukan hal-hal yang baru dan berbeda', 'Saya menceritakan bagaimana saya berhasil melakukan sesuatu', 'Z', 'X'),
(28, 'Bila saya betul, saya suka mempertahankannya', 'Saya ingin diterima dan diakui dalam suatu kelompok', 'K', 'B'),
(29, 'Saya berusaha untuk tidak menjadi seorang yang berbeda', 'Saya berusaha untuk sekali-sekali bersama orang lain', 'F', 'O'),
(30, 'Saya senang diberi tahu bagaimana melakukan suatu pekerjaan', 'Saya mudah bosan', 'W', 'Z'),
(31, 'Saya bekerja keras', 'Saya banyak berfikir dan membuat rencana', 'G', 'R'),
(32, 'Saya memimpin kelompok', 'Detail (hal-hal kecil) menarik bagi saya', 'L', 'D'),
(33, 'Saya mengambil keputusan secara mudah dan cepat', 'Saya menyimpan barang-barang saya secara rapih dan teratur', 'I', 'C'),
(34, 'Biasanya saya bekerja dengan tergesa-gesa', 'Saya jarang marah atau bersedih', 'T', 'E'),
(35, 'Saya ingin menjadi bagian dari kelompok', 'Saya ingin melakukan satu pekerjaan pada satu saat', 'B', 'N'),
(36, 'Saya berusaha berteman secara akrab', 'Saya berusaha sangat keras untuk menjadi yang terbaik', 'O', 'A'),
(37, 'Saya ingin menjadi bagian dari suatu kelompok', 'Saya berusaha menjadi yang terbaik', 'Z', 'P'),
(38, 'Saya menyukai perdebatan', 'Saya suka mendapatkan perhatian', 'K', 'X'),
(39, 'Saya suka mendukung orang-orang yang menjadi atasan saya', 'Saya tertarik menjadi bagian dari kelompok', 'F', 'B'),
(40, 'Saya suka mengikuti peraturan dengan hati-hati', 'Saya suka orang mengenal saya dengan baik', 'W', 'O'),
(41, 'Saya berusaha keras sekali', 'Saya sangat ramah', 'G', 'S'),
(42, 'Orang menilai saya seorang pemimpin yang baik', 'Saya berfikir panjang dan hati-hati', 'L', 'R'),
(43, 'Saya sering mengambil resiko/coba-coba', 'Saya sering mengurus hal-hal kecil/detail', 'I', 'D'),
(44, 'Orang berpendapat bahwa saya bekerja cepat', 'Saya sering mengurus hal-hal kecil/detail', 'T', 'C'),
(45, 'Saya senang mengikuti pertandingan dan olah raga', 'Saya mempunyai pribadi yang menyenangkan', 'V', 'E'),
(46, 'Saya senang jika orang dekat', 'Saya mempunyai pribadi yang menyenangkan', 'O', 'N'),
(47, 'Saya senang bereksperimen dan mencoba hal-hal baru', 'Saya suka melaksanakan suatu pekerjaan sulit dengan baik', 'O', 'A'),
(48, 'Saya suka diperlakukan secara adil', 'Saya suka memberi tahu orang lain bagaimana melaksanakan suatu pekerjaan', 'Z', 'P'),
(49, 'Saya suka melakukan apa yang diharapkan oleh saya', 'Saya suka memperoleh perhatian', 'K', 'X'),
(50, 'Saya suka petunjuk-petunjuk terperinci untuk melaksanakan suatu tugas', 'Saya senang berada bersama orang lain', 'W', 'B'),
(51, 'Saya selalu berusaha menyelesaikan pekerjaan secara sempurna', 'Orang mengatakan bahwa saya tidak mengenal lelah', 'G', 'V'),
(52, 'Saya tipe pemimpin', 'Saya mudah berteman', 'L', 'S'),
(53, 'Saya selalu berspekulasi', 'Saya banyak sekali berfikir', 'I', 'R'),
(54, 'Saya bekerja dengan kecepatan teratur dan tetap', 'Saya senang bekerja dengan hal-hal kecil/terperinci', 'T', 'D'),
(55, 'Saya bersemangat untuk mengikuti berbagai pertandingan dalam olah raga', 'Saya mengatur dan menyimpan barang-barang saya secara rapi dan teratur', 'V', 'C'),
(56, 'Saya dapat bergaul secara baik dengan semua orang', 'Saya adalah seorang yang mempunyai pembawaan tenang', 'S', 'E'),
(57, 'Saya ingin bertemu dengan orang-orang baru dan melakukan hal-hal baru', 'Saya selalu ingin menyelesaikan pekerjaan yang telah saya mulai', 'Z', 'N'),
(58, 'Saya biasanya mempertahankan pendapat yang saya yakini', 'Saya biasanya suka bekerja keras', 'K', 'A'),
(59, 'Saya suka saran-saran dari orang yang saya kagumi', 'Saya senang diserahi tanggung jawab atas kelompok orang', 'F', 'P'),
(60, 'Saya biarkan diri saya banyak dipengaruhi orang lain', 'Saya suka jika mendapat banyak perhatian', 'W', 'X'),
(61, 'Saya berusaha bekerja keras', 'Saya mengerjakan sesuatu secara cepat', 'G', 'C'),
(62, 'Apabila saya bicara, kelompok mendengarkan', 'Saya terampil menggunakan perkakas atau alat-alat', 'L', 'V'),
(63, 'Saya lambat dalam membina hubungan', 'Saya lambat dalam mengambil keputusan', 'I', 'S'),
(64, 'Saya biasanya makan secara cepat', 'Saya suka membaca', 'T', 'R'),
(65, 'Saya suka pekerjaan dimana saya banyak bergerak', 'Saya suka pekerjaan yang harus dilakukan secara hati-hati', 'V', 'D'),
(66, 'Saya mencari teman sebanyak mungkin', 'Apa yang sudah saya simpan, akan mudah saya temukan kembali', 'S', 'C'),
(67, 'Saya merencanakan jauh-jauh hari berikutnya', 'Saya selalu menyenangkan', 'R', 'E'),
(68, 'Saya mempertahankan nama baik saya dengan bangga', 'Saya terus menekuni suatu masalah sampai selesai', 'K', 'N'),
(69, 'Saya suka mendukung orang-orang yang saya kagumi', 'Saya ingin sukses', 'F', 'A'),
(70, 'Saya suka orang lain yang memutuskan untuk kelompok', 'Saya suka membuat keputusan untuk kelompok', 'W', 'P'),
(71, 'Saya selalu berusaha bekerja keras', 'Saya mengambil keputusan secara cepat dan mudah', 'G', 'I'),
(72, 'Kelompok biasanya melakukan apa yang saya inginkan', 'Saya biasa terburu-buru', 'L', 'T'),
(73, 'Saya sering merasa lelah', 'Saya lamban dalam menentukan sikap', 'I', 'V'),
(74, 'Saya bekerja cepat', 'Saya mudah berteman', 'T', 'S'),
(75, 'Saya biasanya mempunyai semangat dan tenaga', 'Saya banyak menghabiskan waktu dengan berfikir', 'V', 'R'),
(76, 'Saya sangat ramah terhadap orang', 'Saya suka pekerjaan yang memerlukan ketelitian', 'S', 'D'),
(77, 'Saya banyak berfikir dan merencanakan', 'Saya menyimpan segala sesuatu pada tempatnya', 'R', 'C'),
(78, 'Saya suka pekerjaan yang harus memperhatikan hal-hal kecil (detail)', 'Saya tidak mudah marah', 'D', 'E'),
(79, 'Saya suka mengikuti orang yang saya kagumi', 'Saya selalu menyelesaikan pekerjaan yang telah saya mulai', 'V', 'N'),
(80, 'Saya suka petunjuk-petunjuk yang jelas', 'Saya suka bekerja keras', 'W', 'A'),
(81, 'Saya mengejar apa yang saya inginkan', 'Saya seorang pemimpin yang baik', 'G', 'L'),
(82, 'Saya dapat membuat orang lain bekerja sesuai dengan yang saya inginkan', 'Saya seorang yang tergolong santai dan beruntung', 'L', 'I'),
(83, 'Saya mengambil keputusan secara mudah dan amat cepat', 'Saya bicara secara cepat', 'I', 'T'),
(84, 'Saya biasanya bekerja cepat', 'Saya pemimpin yang baik', 'T', 'V'),
(85, 'Saya tidak suka bertemu dengan orang', 'Saya cepat merasa lelah', 'V', 'S'),
(86, 'Saya mempunyai banyak sekali teman', 'Saya banyak menghabiskan waktu dengan berfikir', 'S', 'R'),
(87, 'Saya suka berjalan dengan teori', 'Saya suka bekerja dengan hal-hal yang terperinci', 'R', 'D'),
(88, 'Saya menikmati pekerjaan yang melibatkan hal-hal kecil (detail)', 'Saya suka mengorganisir pekerjaan saya', 'D', 'C'),
(89, 'Saya menaruh barang pada tempatnya', 'Saya selalu menyenangkan', 'C', 'E'),
(90, 'Saya suka diberitahu tentang apa yang perlu dilakukan', 'Saya harus menyelesaikan apa yang saya mulai', 'W', 'N');
