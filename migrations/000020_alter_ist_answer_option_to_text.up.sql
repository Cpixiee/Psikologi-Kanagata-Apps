-- GE (61-76) now accepts free-text answers, so answer_option must store text
ALTER TABLE ist_answers
  ALTER COLUMN answer_option TYPE VARCHAR(255);

