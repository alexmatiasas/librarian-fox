-- ═══════════════════════════════════════════════════════
-- lfx schema testing script
--
-- NOTE: This script references specific test files that are NOT in the repo
-- (too large). It serves as documentation of how the schema works.
-- See docs/schema.md for the full schema reference.
--
-- To run: sqlite3 /tmp/lfx_test.db < internal/manifest/migrations/001_initial.sql
--         sqlite3 /tmp/lfx_test.db < testdata/test_schema.sql
-- ═══════════════════════════════════════════════════════

-- Configuration: readable output
.headers on
.mode column
.width 60 10

-- Enable foreign keys (SQLite has this OFF by default)
PRAGMA foreign_keys = ON;

-- ═══════════════════════════════════════════════════════
-- SECTION 1: BASIC CRUD
-- Insert documents, classifications, tags, organization, duplicated
-- Verify triggers and delete on cascade
-- ═══════════════════════════════════════════════════════
SELECT '═══════════════════════════════════════════════════════' AS '';
SELECT 'SECTION 1: BASIC CRUD' AS '';
SELECT '═══════════════════════════════════════════════════════' AS '';

-- 1.1 Insert 12 documents (real SHA-256 from documents)
INSERT INTO documents (id, source_path, file_type, file_size, page_count, has_text_layer, language)
VALUES
    ('9ce4722dca9650c6ce47bc082318f031bebec998b0803daca9f29bed6c44cc27',
     '/Users/test/testdata/Libros/Physics/Classical Electrodynamics - Jackson.pdf',
     'pdf', 26623622, 657, FALSE, 'en'),
    ('bead22240b7be65e2bda3613b7b0083f6383e849944dc9e1b0204f020cb79b97',
     '/Users/test/testdata/Libros/Physics/John David Jackson - Classical electrodynamics-Wiley (1999).djvu',
     'djvu', 9818885, NULL, FALSE, 'en'),
    ('df562a0a96ca3fcd8e7fdb60fa5994e8eea5925dc60bd1c12abb30a39cdee3fa',
     '/Users/test/testdata/Manuales/Physics/56406005-Jackson-Solutions.pdf',
     'pdf', 11670737, NULL, TRUE, 'en'),
    ('e181911c57714296cca6110eb72571c92e4b15d31b4d8ad4bcd07dfb502319c',
     '/Users/test/testdata/Documentos Personales/acta_nacimiento.pdf',
     'pdf', 533530, 1, TRUE, 'es'),
    ('6089eaa5c66d9c58ed51995c30b9291e13aae219b8357eee8415f4f1f7dc50e9',
     '/Users/test/testdata/Notas/Physics/qft3.pdf',
     'pdf', 85397, NULL, TRUE, 'es'),
    ('4d369d231efc6672270e1f06b511c5c3ee15bd471ed9d27d83cd276abc968a10',
     '/Users/test/testdata/Notas/Physics/EjerciciosParticulas.pdf',
     'pdf', 174174, NULL, TRUE, 'es'),
    ('c17195960ba23e33af19322a34a613cf8a90e7848d6ee499666c4c6003ccef72',
     '/Users/test/testdata/Papers/Physics/Tan et al. - 2019 - Some characteristic parameters of proton from the bag model.pdf',
     'pdf', 106787, 4, TRUE, 'en'),
    ('9c31359e94b132ded76643fabbbc6715e097030cd5bd9b5330c8b26f2d48bdc6',
     '/Users/test/testdata/Tesis/Physics/Tesis.pdf',
     'pdf', 4457356, NULL, TRUE, 'es'),
    ('d6f2a0178806803507be557c65eefa5439e0b2312315458c96eb69a445b33173',
     '/Users/test/testdata/Libros/Statistics/Bayesian Data Analysis.pdf',
     'pdf', 66473875, NULL, TRUE, 'en'),
    ('7dafdf75f2cba6187231a8f1b688f2242baa61530bc89006a6a079a0b740cfba',
     '/Users/test/testdata/Libros/Machine Learning/Hands-On_Machine_Learning_with_Scikit-Learn-Keras-and-TensorFlow-2nd-Edition-Aurelien-Geron.pdf',
     'pdf', 60050105, 851, TRUE, 'en'),
    ('edfb143de104642e1dd88288b6b630381fa81875b7d0b51df1baaca760a9643b',
     '/Users/test/testdata/Libros/Engineering/(Cambridge Mathematical Library) G. K. Batchelor - An Introduction to Fluid Dynamics -Cambridge University Press (2000).djvu',
     'djvu', 8476701, NULL, FALSE, 'en'),
    ('0e2517d7f62b908fd3fba49522e946831b28970b2fc47256dfad373ae6c93cb4',
     '/Users/test/testdata/Libros/Grammar/Jane Straus, Lester Kaufman, Tom Stern - The Blue Book of Grammar and Punctuation_ An Easy-to-Use Guide with Clear Rules, Real-World Examples, and Reproducible Quizzes-Wiley (2014).epub',
     'epub', 1605671, NULL, TRUE, 'en');

SELECT COUNT(*) || ' documents inserted' AS result FROM documents;

-- 1.2 Insert classifications (one by document)
INSERT INTO classifications (document_id, source, model, tipo, tipo_original, subject, subject_hierarchy, autor, titulo, anio, editorial, description, confidence, accepted)
VALUES
    -- Jackson PDF (scan, needs OCR)
    ('9ce4722dca9650c6ce47bc082318f031bebec998b0803daca9f29bed6c44cc27',
     'llm', 'qwen3:8b', 'libro', 'Libro', 'Physics',
     '["Physics", "Electrodynamics"]',
     'John David Jackson', 'Classical Electrodynamics', '1999', 'Wiley',
     'Classic graduate textbook on electrodynamics', 0.95, TRUE),
    -- Jackson DjVu (same book, different format)
    ('bead22240b7be65e2bda3613b7b0083f6383e849944dc9e1b0204f020cb79b97',
     'llm', 'qwen3:8b', 'libro', 'Libro', 'Physics',
     '["Physics", "Electrodynamics"]',
     'John David Jackson', 'Classical Electrodynamics', '1999', 'Wiley',
     'Classic graduate textbook on electrodynamics (DjVu format)', 0.95, TRUE),
    -- Solutions manual
    ('df562a0a96ca3fcd8e7fdb60fa5994e8eea5925dc60bd1c12abb30a39cdee3fa',
     'llm', 'qwen3:8b', 'manual', 'Manual', 'Physics',
     '["Physics", "Electrodynamics"]',
     'Unknown', 'Solutions to Classical Electrodynamics', '1999', 'Wiley',
     'Solutions manual for Jackson''s Classical Electrodynamics', 0.90, TRUE),
    -- Birth certificate (personal document)
    ('e181911c57714296cca6110eb72571c92e4b15d31b4d8ad4bcd07dfb502319c',
     'heuristic', NULL, 'personal', 'Documento personal', NULL,
     NULL,
     NULL, 'Acta de nacimiento', NULL, NULL,
     'Documento personal de nacimiento', 1.0, TRUE),
    -- QFT homework (notes)
    ('6089eaa5c66d9c58ed51995c30b9291e13aae219b8357eee8415f4f1f7dc50e9',
     'llm', 'qwen3:8b', 'notas', 'Notas', 'Physics',
     '["Physics", "Quantum Field Theory"]',
     NULL, 'Tercera tarea de Teoría Cuántica de Campos', NULL, NULL,
     'Tarea de homework de QFT', 0.85, TRUE),
    -- Ejercicios de partículas (notes, Spanish)
    ('4d369d231efc6672270e1f06b511c5c3ee15bd471ed9d27d83cd276abc968a10',
     'llm', 'qwen3:8b', 'notas', 'Notas', 'Physics',
     '["Physics", "Particle Physics"]',
     'Pablo Roig Garcés', 'Ejercicios de Física de Partículas Elementales', NULL, NULL,
     'Ejercicios de partículas elementales', 0.85, TRUE),
    -- Tan et al. paper
    ('c17195960ba23e33af19322a34a613cf8a90e7848d6ee499666c4c6003ccef72',
     'llm', 'qwen3:8b', 'paper', 'Paper', 'Physics',
     '["Physics", "Nuclear Physics"]',
     'Tan et al.', 'Some Characteristic Parameters of Proton from the Bag Model', '2019', NULL,
     'Academic paper on proton parameters', 0.92, TRUE),
    -- Tesis
    ('9c31359e94b132ded76643fabbbc6715e097030cd5bd9b5330c8b26f2d48bdc6',
     'llm', 'qwen3:8b', 'tesis', 'Tesis', 'Physics',
     '["Physics"]',
     'Unknown', 'Tesis de Ingeniería en Física Aplicada', NULL,
     'Universidad Tecnológica de la Mixteca',
     'Tesis de grado en Ingeniería en Física Aplicada', 0.88, TRUE),
    -- Bayesian Data Analysis
    ('d6f2a0178806803507be557c65eefa5439e0b2312315458c96eb69a445b33173',
     'llm', 'qwen3:8b', 'libro', 'Libro', 'Statistics',
     '["Statistics", "Bayesian Statistics"]',
     'Andrew Gelman et al.', 'Bayesian Data Analysis', '2013', 'CRC Press',
     'Comprehensive textbook on Bayesian statistical methods', 0.95, TRUE),
    -- Hands-On ML
    ('7dafdf75f2cba6187231a8f1b688f2242baa61530bc89006a6a079a0b740cfba',
     'llm', 'qwen3:8b', 'libro', 'Libro', 'Machine Learning',
     '["Computer Science", "Machine Learning"]',
     'Aurélien Géron', 'Hands-On Machine Learning with Scikit-Learn, Keras, and TensorFlow', '2019', 'O''Reilly',
     'Practical guide to ML with Python', 0.95, TRUE),
    -- Batchelor Fluid Dynamics
    ('edfb143de104642e1dd88288b6b630381fa81875b7d0b51df1baaca760a9643b',
     'llm', 'qwen3:8b', 'libro', 'Libro', 'Engineering',
     '["Engineering", "Fluid Dynamics"]',
     'G. K. Batchelor', 'An Introduction to Fluid Dynamics', '2000', 'Cambridge University Press',
     'Classic textbook on fluid dynamics', 0.95, TRUE),
    -- Blue Book of Grammar (EPUB)
    ('0e2517d7f62b908fd3fba49522e946831b28970b2fc47256dfad373ae6c93cb4',
     'llm', 'qwen3:8b', 'libro', 'Libro', 'Grammar',
     '["Language", "Grammar"]',
     'Jane Straus', 'The Blue Book of Grammar and Punctuation', '2014', 'Wiley',
     'Grammar and punctuation guide', 0.90, TRUE);

SELECT COUNT(*) || ' classifications inserted' AS result FROM classifications;

-- 1.3 Verify updated_at is updated (trigger)
SELECT '--- Verificando trigger updated_at ---' AS '';
UPDATE documents SET file_size = 26623623 WHERE id = '9ce4722dca9650c6ce47bc082318f031bebec998b0803daca9f29bed6c44cc27';
SELECT id, file_size, updated_at FROM documents WHERE id = '9ce4722dca9650c6ce47bc082318f031bebec998b0803daca9f29bed6c44cc27';

-- 1.4 Insert tags and document_tags
INSERT INTO document_tags (document_id, tag_id)
VALUES
    ('9ce4722dca9650c6ce47bc082318f031bebec998b0803daca9f29bed6c44cc27', 1),  -- Jackson → to_read
    ('7dafdf75f2cba6187231a8f1b688f2242baa61530bc89006a6a079a0b740cfba', 3),  -- Geron → finished
    ('e181911c57714296cca6110eb72571c92e4b15d31b4d8ad4bcd07dfb502319c', 7),  -- acta → favorite
    ('c17195960ba23e33af19322a34a613cf8a90e7848d6ee499666c4c6003ccef72', 4); -- Tan → reference

SELECT COUNT(*) || ' document_tags inserted' AS result FROM document_tags;

-- 1.5 Verify cascade delete (Delete an document temporary)
-- First we create an empty file to delete
INSERT INTO documents (id, source_path, file_type, file_size, language)
VALUES ('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
        '/tmp/temp.pdf', 'pdf', 1000, 'en');
INSERT INTO classifications (document_id, source, tipo, titulo, accepted)
VALUES ('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
        'llm', 'otro', 'Temporary', TRUE);
INSERT INTO document_tags (document_id, tag_id)
VALUES ('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 1);

SELECT '--- Before CASCADE delete ---' AS '';
SELECT COUNT(*) AS docs FROM documents;
SELECT COUNT(*) AS classifications FROM classifications;
SELECT COUNT(*) AS doc_tags FROM document_tags;

DELETE FROM documents WHERE id = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';

SELECT '--- After CASCADE delete ---' AS '';
SELECT COUNT(*) AS docs FROM documents;
SELECT COUNT(*) AS classifications FROM classifications;
SELECT COUNT(*) AS doc_tags FROM document_tags;

-- ═══════════════════════════════════════════════════════
-- SECTION 2: FULL-TEXT SEARCH (FTS5)
-- Search by word, prefix, multiple fields
-- Verify sync triggers
-- ═══════════════════════════════════════════════════════
SELECT '' AS '';
SELECT '═══════════════════════════════════════════════════════' AS '';
SELECT 'SECTION 2: FULL-TEXT SEARCH (FTS5)' AS '';
SELECT '═══════════════════════════════════════════════════════' AS '';

-- 2.1 Search "Jackson" → 2 results (PDF + DjVu)
SELECT '--- Search "Jackson" ---' AS '';
SELECT c.titulo, c.autor FROM classifications_fts f
JOIN classifications c ON f.rowid = c.id
WHERE classifications_fts MATCH 'Jackson';

-- 2.2 Search "machine learning" → 1 result (Geron)
SELECT '--- Search "machine learning" ---' AS '';
SELECT c.titulo, c.autor FROM classifications_fts f
JOIN classifications c ON f.rowid = c.id
WHERE classifications_fts MATCH 'machine learning';

-- 2.3 Search "quantum" → 0 results (no QFT in english)
SELECT '--- Search "quantum" (debería dar 0) ---' AS '';
SELECT COUNT(*) AS results FROM classifications_fts
WHERE classifications_fts MATCH 'quantum';

-- 2.4 Search "partículas" → 1 result (EjerciciosParticulas, ES)
SELECT '--- Search "partículas" ---' AS '';
SELECT c.titulo FROM classifications_fts f
JOIN classifications c ON f.rowid = c.id
WHERE classifications_fts MATCH 'partículas';

-- 2.5 Search by prefix: "electrod*" → 2 results
SELECT '--- Search by prefix "electrod*" ---' AS '';
SELECT c.titulo FROM classifications_fts f
JOIN classifications c ON f.rowid = c.id
WHERE classifications_fts MATCH 'electrod*';

-- 2.6 Search OR: "bayesian OR statistics" → 1 result
SELECT '--- Search "bayesian OR statistics" ---' AS '';
SELECT c.titulo FROM classifications_fts f
JOIN classifications c ON f.rowid = c.id
WHERE classifications_fts MATCH 'bayesian OR statistics';

-- 2.7 Verify FTS sync after UPDATE
SELECT '--- Verify FTS sync after UPDATE ---' AS '';
-- Update document title
UPDATE classifications SET titulo = 'Classical Electrodynamics (Updated Title)'
WHERE document_id = '9ce4722dca9650c6ce47bc082318f031bebec998b0803daca9f29bed6c44cc27';
-- Search with the new title
SELECT c.titulo FROM classifications_fts f
JOIN classifications c ON f.rowid = c.id
WHERE classifications_fts MATCH 'Updated';
-- Restore original title
UPDATE classifications SET titulo = 'Classical Electrodynamics'
WHERE document_id = '9ce4722dca9650c6ce47bc082318f031bebec998b0803daca9f29bed6c44cc27';

-- 2.8 Verify FTS sync after DELETE
SELECT '--- Verify FTS sync after DELETE ---' AS '';
-- Create temporary document to delete
INSERT INTO documents (id, source_path, file_type, file_size, language)
VALUES ('bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
        '/tmp/temp_fts.pdf', 'pdf', 1000, 'en');
INSERT INTO classifications (document_id, source, tipo, titulo, accepted)
VALUES ('bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
        'llm', 'otro', 'Temporary FTS Test', TRUE);
-- Verify it can be searched
SELECT COUNT(*) AS before_delete FROM classifications_fts
WHERE classifications_fts MATCH 'Temporary';
-- Delete
DELETE FROM documents WHERE id = 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb';
-- Verify it cannot be searched anymore
SELECT COUNT(*) AS after_delete FROM classifications_fts
WHERE classifications_fts MATCH 'Temporary';

-- ═══════════════════════════════════════════════════════
-- SECTION 3: COMPLETE PIPELINE
-- Simulate flow: scan → classify → approve → OCR → apply → dedup
-- ═══════════════════════════════════════════════════════
SELECT '' AS '';
SELECT '═══════════════════════════════════════════════════════' AS '';
SELECT 'SECTION 3: COMPLETE PIPELINE' AS '';
SELECT '═══════════════════════════════════════════════════════' AS '';

-- 3.1 Mark documents that need OCR (Phase 3)
SELECT '--- Marking documents for OCR ---' AS '';
UPDATE documents SET needs_ocr = TRUE, quality_flags = '["blur","skew"]'
WHERE id = '9ce4722dca9650c6ce47bc082318f031bebec998b0803daca9f29bed6c44cc27';
UPDATE documents SET needs_ocr = TRUE, quality_flags = '["low_contrast"]'
WHERE id = 'edfb143de104642e1dd88288b6b630381fa81875b7d0b51df1baaca760a9643b';

SELECT COUNT(*) || ' Documents marked for OCR' AS result
FROM documents WHERE needs_ocr = TRUE;

-- 3.2 Insert organization records (Phase 4: apply)
SELECT '--- Inserting organization records ---' AS '';
INSERT INTO organization (document_id, source_path, target_path, backup_path)
VALUES
    ('9ce4722dca9650c6ce47bc082318f031bebec998b0803daca9f29bed6c44cc27',
     '/Users/test/testdata/Libros/Physics/Classical Electrodynamics - Jackson.pdf',
     '/Users/test/Biblioteca/Libros/Physics/Jackson - Classical Electrodynamics (1999).pdf',
     '/Users/test/Biblioteca/.backups/2024-01-15/Jackson - Classical Electrodynamics (1999).pdf'),
    ('bead22240b7be65e2bda3613b7b0083f6383e849944dc9e1b0204f020cb79b97',
     '/Users/test/testdata/Libros/Physics/John David Jackson - Classical electrodynamics-Wiley (1999).djvu',
     '/Users/test/Biblioteca/Libros/Physics/Jackson - Classical Electrodynamics (1999).djvu',
     '/Users/test/Biblioteca/.backups/2024-01-15/Jackson - Classical Electrodynamics (1999).djvu'),
    ('7dafdf75f2cba6187231a8f1b688f2242baa61530bc89006a6a079a0b740cfba',
     '/Users/test/testdata/Libros/Machine Learning/Hands-On ML...Geron.pdf',
     '/Users/test/Biblioteca/Libros/Machine Learning/Géron - Hands-On ML (2019).pdf',
     '/Users/test/Biblioteca/.backups/2024-01-15/Géron - Hands-On ML (2019).pdf');

SELECT COUNT(*) || ' organization records inserted' AS result FROM organization;

-- 3.3 Insertar duplicates (Jackson PDF + DjVu)
SELECT '--- Insertando duplicados ---' AS '';
INSERT INTO duplicates (document_id, duplicate_of, similarity, reason)
VALUES
    ('bead22240b7be65e2bda3613b7b0083f6383e849944dc9e1b0204f020cb79b97',
     '9ce4722dca9650c6ce47bc082318f031bebec998b0803daca9f29bed6c44cc27',
     0.85, 'different_edition'),
    ('9ce4722dca9650c6ce47bc082318f031bebec998b0803daca9f29bed6c44cc27',
     'bead22240b7be65e2bda3613b7b0083f6383e849944dc9e1b0204f020cb79b97',
     0.85, 'different_edition');

SELECT COUNT(*) || ' duplicates inserted' AS result FROM duplicates;

-- ═══════════════════════════════════════════════════════
-- SECTION 4: EDGE CASES E INTEGRIDAD
-- CHECK constraints, unique constraints, tildes/ñ, NULLs
-- ═══════════════════════════════════════════════════════
SELECT '' AS '';
SELECT '═══════════════════════════════════════════════════════' AS '';
SELECT 'SECTION 4: EDGE CASES E INTEGRIDAD' AS '';
SELECT '═══════════════════════════════════════════════════════' AS '';

-- 4.1 CHECK constraint: file_type inválido (debe fallar)
SELECT '--- Test: file_type inválido (debe fallar) ---' AS '';
-- Esto debe fallar con: CHECK constraint failed
INSERT OR IGNORE INTO documents (id, source_path, file_type, file_size, language)
VALUES ('cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc',
        '/tmp/bad.pdf', 'invalid_type', 1000, 'en');
SELECT CASE WHEN COUNT(*) = 0 THEN 'PASS: Invalid file_type rejected'
            ELSE 'FAIL: Invalid file_type accepted' END AS result
FROM documents WHERE id = 'cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc';

-- 4.2 UNIQUE constraint: dos accepted classifications para el mismo documento (debe fallar)
SELECT '--- Test: dos accepted para mismo documento (debe fallar) ---' AS '';
INSERT OR IGNORE INTO classifications (document_id, source, tipo, titulo, accepted)
VALUES ('9ce4722dca9650c6ce47bc082318f031bebec998b0803daca9f29bed6c44cc27',
        'llm', 'otro', 'Duplicate accepted', TRUE);
SELECT CASE WHEN (SELECT COUNT(*) FROM classifications
                  WHERE document_id = '9ce4722dca9650c6ce47bc082318f031bebec998b0803daca9f29bed6c44cc27'
                  AND accepted = TRUE) = 1
            THEN 'PASS: Only one accepted classification per document'
            ELSE 'FAIL: Multiple accepted classifications' END AS result;

-- 4.3 Spanish characters: tildes y ñ
SELECT '--- Test: caracteres españoles ---' AS '';
SELECT titulo, autor FROM classifications
WHERE document_id = '4d369d231efc6672270e1f06b511c5c3ee15bd471ed9d27d83cd276abc968a10';
SELECT CASE WHEN titulo LIKE '%Física%' THEN 'PASS: Tildes stored correctly'
            ELSE 'FAIL: Tildes corrupted' END AS result
FROM classifications
WHERE document_id = '4d369d231efc6672270e1f06b511c5c3ee15bd471ed9d27d83cd276abc968a10';

-- 4.4 CASCADE delete completo
SELECT '--- Test: CASCADE delete completo ---' AS '';
-- Crear documento con todo vinculado
INSERT INTO documents (id, source_path, file_type, file_size, language)
VALUES ('dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd',
        '/tmp/cascade_test.pdf', 'pdf', 1000, 'en');
INSERT INTO classifications (document_id, source, tipo, titulo, accepted)
VALUES ('dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd',
        'llm', 'otro', 'Cascade Test', TRUE);
INSERT INTO document_tags (document_id, tag_id)
VALUES ('dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd', 1);
INSERT INTO organization (document_id, source_path, target_path)
VALUES ('dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd',
        '/tmp/cascade_test.pdf', '/tmp/cascade_test.pdf');

-- Verificar que todo existe antes
SELECT 'Before delete: ' ||
       (SELECT COUNT(*) FROM documents WHERE id = 'dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd') || ' docs, ' ||
       (SELECT COUNT(*) FROM classifications WHERE document_id = 'dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd') || ' class, ' ||
       (SELECT COUNT(*) FROM document_tags WHERE document_id = 'dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd') || ' tags, ' ||
       (SELECT COUNT(*) FROM organization WHERE document_id = 'dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd') || ' org'
AS result;

DELETE FROM documents WHERE id = 'dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd';

SELECT 'After delete: ' ||
       (SELECT COUNT(*) FROM documents WHERE id = 'dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd') || ' docs, ' ||
       (SELECT COUNT(*) FROM classifications WHERE document_id = 'dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd') || ' class, ' ||
       (SELECT COUNT(*) FROM document_tags WHERE document_id = 'dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd') || ' tags, ' ||
       (SELECT COUNT(*) FROM organization WHERE document_id = 'dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd') || ' org'
AS result;

-- 4.5 Campos NULL opcionales
SELECT '--- Test: campos NULL opcionales ---' AS '';
SELECT CASE WHEN COUNT(*) > 0 THEN 'PASS: NULL optional fields handled'
            ELSE 'FAIL: NULL fields caused error' END AS result
FROM documents WHERE page_count IS NULL;

-- 4.6 Strings vacíos
SELECT '--- Test: strings vacíos ---' AS '';
-- Crear documento fresco para no violar unique constraint de accepted
INSERT INTO documents (id, source_path, file_type, file_size, language)
VALUES ('eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee',
        '/tmp/empty_strings_test.pdf', 'pdf', 1000, 'en');
INSERT INTO classifications (document_id, source, tipo, titulo, description, accepted)
VALUES ('eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee',
        'manual', 'personal', '', '', TRUE);
SELECT CASE WHEN COUNT(*) > 0 THEN 'PASS: Empty strings accepted'
            ELSE 'FAIL: Empty strings rejected' END AS result
FROM classifications
WHERE document_id = 'eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee'
AND titulo = '';
-- Limpiar
DELETE FROM classifications
WHERE document_id = 'eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee';
DELETE FROM documents
WHERE id = 'eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee';

-- ═══════════════════════════════════════════════════════
-- SECTION 5: QUERIES DEL MUNDO REAL
-- Queries que lfx ejecutaría en producción
-- ═══════════════════════════════════════════════════════
SELECT '' AS '';
SELECT '═══════════════════════════════════════════════════════' AS '';
SELECT 'SECTION 5: QUERIES DEL MUNDO REAL' AS '';
SELECT '═══════════════════════════════════════════════════════' AS '';

-- 5.1 ¿Cuántos documentos por tipo?
SELECT '--- Documentos por tipo ---' AS '';
SELECT k.label_es AS tipo, COUNT(*) AS cantidad
FROM classifications c
JOIN known_types k ON c.tipo = k.key
WHERE c.accepted = TRUE
GROUP BY c.tipo
ORDER BY cantidad DESC;

-- 5.2 ¿Cuántos necesitan OCR?
SELECT '--- Documentos que necesitan OCR ---' AS '';
SELECT COUNT(*) AS necesita_ocr FROM documents WHERE needs_ocr = TRUE;

-- 5.3 Buscar por título (búsqueda parcial)
SELECT '--- Buscar por título "Electrodynamics" ---' AS '';
SELECT c.titulo, c.autor FROM classifications c
WHERE c.accepted = TRUE AND c.titulo LIKE '%Electrodynamics%';

-- 5.4 Ver documentos por subject
SELECT '--- Documentos por subject ---' AS '';
SELECT subject, COUNT(*) AS cantidad
FROM classifications
WHERE accepted = TRUE AND subject IS NOT NULL
GROUP BY subject
ORDER BY cantidad DESC;

-- 5.5 ¿Qué documentos están en la librería?
SELECT '--- Documentos en la librería ---' AS '';
SELECT o.target_path, c.titulo FROM organization o
JOIN classifications c ON o.document_id = c.document_id
WHERE c.accepted = TRUE;

-- 5.6 ¿Cuántos duplicados tengo?
SELECT '--- Duplicados ---' AS '';
SELECT d.similarity, d.reason,
       c1.titulo AS titulo_1, c2.titulo AS titulo_2
FROM duplicates d
JOIN classifications c1 ON d.document_id = c1.document_id
JOIN classifications c2 ON d.duplicate_of = c2.document_id;

-- 5.7 Búsqueda por idioma
SELECT '--- Documentos por idioma ---' AS '';
SELECT language, COUNT(*) AS cantidad
FROM documents
GROUP BY language;

-- 5.8 Documentos por tamaño
SELECT '--- Documentos más grandes (top 5) ---' AS '';
SELECT c.titulo, d.file_size / 1048576 AS size_mb
FROM documents d
JOIN classifications c ON d.id = c.document_id
WHERE c.accepted = TRUE
ORDER BY d.file_size DESC
LIMIT 5;

-- ═══════════════════════════════════════════════════════
-- SECTION 6: VERIFICACIÓN FINAL
-- Conteos, integridad, FTS sync
-- ═══════════════════════════════════════════════════════
SELECT '' AS '';
SELECT '═══════════════════════════════════════════════════════' AS '';
SELECT 'SECTION 6: VERIFICACIÓN FINAL' AS '';
SELECT '═══════════════════════════════════════════════════════' AS '';

-- 6.1 Conteos por tabla
SELECT '--- Conteos finales ---' AS '';
SELECT 'documents' AS tabla, COUNT(*) AS registros FROM documents
UNION ALL
SELECT 'known_types', COUNT(*) FROM known_types
UNION ALL
SELECT 'subject_groups', COUNT(*) FROM subject_groups
UNION ALL
SELECT 'classifications', COUNT(*) FROM classifications
UNION ALL
SELECT 'tags', COUNT(*) FROM tags
UNION ALL
SELECT 'document_tags', COUNT(*) FROM document_tags
UNION ALL
SELECT 'organization', COUNT(*) FROM organization
UNION ALL
SELECT 'duplicates', COUNT(*) FROM duplicates
UNION ALL
SELECT '_migrations', COUNT(*) FROM _migrations;

-- 6.2 Verificar que FTS5 está sincronizado
SELECT '--- FTS5 sync check ---' AS '';
SELECT CASE WHEN (SELECT COUNT(*) FROM classifications) = (SELECT COUNT(*) FROM classifications_fts)
            THEN 'PASS: FTS5 synchronized'
            ELSE 'FAIL: FTS5 out of sync' END AS result;

-- 6.3 Verificar unique constraints
SELECT '--- Unique constraints check ---' AS '';
SELECT CASE WHEN COUNT(*) = 0
            THEN 'PASS: No duplicate document IDs'
            ELSE 'FAIL: Duplicate document IDs found' END AS result
FROM documents
GROUP BY id HAVING COUNT(*) > 1;

-- 6.4 Verificar que todos los accepted tienen clasificación
SELECT '--- All accepted classifications have documents ---' AS '';
SELECT CASE WHEN COUNT(*) = 0
            THEN 'PASS: All accepted classifications reference existing documents'
            ELSE 'FAIL: Orphaned accepted classifications' END AS result
FROM classifications c
LEFT JOIN documents d ON c.document_id = d.id
WHERE c.accepted = TRUE AND d.id IS NULL;

-- 6.5 Resumen final
SELECT '' AS '';
SELECT '═══════════════════════════════════════════════════════' AS '';
SELECT 'TESTING COMPLETE' AS '';
SELECT '═══════════════════════════════════════════════════════' AS '';
SELECT 'Revisar los resultados arriba. Todos los tests deberían mostrar PASS.' AS '';
