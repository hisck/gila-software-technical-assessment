CREATE ROLE catalog LOGIN PASSWORD 'catalog-local';
CREATE ROLE search LOGIN PASSWORD 'search-local';
CREATE ROLE orders LOGIN PASSWORD 'orders-local';
CREATE ROLE payments LOGIN PASSWORD 'payments-local';

CREATE DATABASE catalog OWNER catalog;
CREATE DATABASE search OWNER search;
CREATE DATABASE orders OWNER orders;
CREATE DATABASE payments OWNER payments;

REVOKE CONNECT ON DATABASE catalog, search, orders, payments, postgres FROM PUBLIC;
