REVOKE ALL ON FUNCTION search.query_documents(uuid, text, integer) FROM veltrix_app;
DROP FUNCTION IF EXISTS search.query_documents(uuid, text, integer);
DROP TYPE IF EXISTS search.document_query_result;
