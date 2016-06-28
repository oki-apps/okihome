-- Copyright 2016 Simon HEGE. All rights reserved.
-- Use of this source code is governed by a MIT-style
-- license that can be found in the LICENSE file.

--
-- PostgreSQL database setup for Okihome
--

SET statement_timeout = 0;
SET lock_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SET check_function_bodies = false;
SET client_min_messages = warning;
SET default_tablespace = '';
SET default_with_oids = false;


CREATE SCHEMA okihome;
SET search_path = okihome, pg_catalog;

CREATE TABLE t_user (
    id text NOT NULL,
    display_name text,
    email text,
    isadmin boolean,
    password text,
    CONSTRAINT c_pk_user PRIMARY KEY (id)
);

CREATE TABLE t_tab (
    id bigserial NOT NULL,
    title text,
    pos integer,
    layout jsonb,
    CONSTRAINT c_pk_tab PRIMARY KEY (id)
);

CREATE TABLE tj_tabaccess (
    tab_id bigserial NOT NULL,
    user_id text NOT NULL,
    CONSTRAINT c_pk_tabaccess PRIMARY KEY (user_id, tab_id),
    CONSTRAINT c_fk_tab FOREIGN KEY (tab_id)
        REFERENCES okihome.t_tab (id) MATCH SIMPLE
        ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT c_fk_user FOREIGN KEY (user_id)
        REFERENCES okihome.t_user (id) MATCH SIMPLE
        ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE TABLE t_widget (
    id bigserial NOT NULL,
    tab_id bigint NOT NULL,
    type text,
    config jsonb,
    CONSTRAINT c_pk_widget PRIMARY KEY (id),
    CONSTRAINT c_fk_widget_tab FOREIGN KEY (tab_id)
        REFERENCES okihome.t_tab (id) MATCH SIMPLE
        ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE TABLE t_account (
    id bigserial NOT NULL,
    user_id text NOT NULL,
    provider text NOT NULL,
    account_id text NOT NULL,
    token jsonb NOT NULL,
    CONSTRAINT c_pk_account PRIMARY KEY (id),
    CONSTRAINT c_fk_account_user FOREIGN KEY (user_id)
        REFERENCES okihome.t_user (id) MATCH SIMPLE
        ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE TABLE t_emailitem (
    account_id bigint NOT NULL,
    guid text NOT NULL,
    title text DEFAULT ''::text NOT NULL,
    read boolean DEFAULT false NOT NULL,
    published timestamp with time zone DEFAULT now() NOT NULL,
    link text DEFAULT ''::text NOT NULL,
    sender text DEFAULT ''::text NOT NULL,
    snippet text DEFAULT ''::text NOT NULL,
    version bigint DEFAULT 0 NOT NULL,
    CONSTRAINT c_pk_emailitem PRIMARY KEY (account_id, guid),
    CONSTRAINT c_fk_emailitem_account FOREIGN KEY (account_id)
        REFERENCES okihome.t_account (id) MATCH SIMPLE
        ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE TABLE t_feed (
    id bigserial NOT NULL,
    url text NOT NULL,
    next_retrieval timestamp with time zone DEFAULT now() NOT NULL,
    title text DEFAULT ''::text NOT NULL,
    CONSTRAINT c_pk_feed PRIMARY KEY (id)
);

CREATE TABLE t_feeditem (
    feed_id bigint NOT NULL,
    guid text NOT NULL,
    title text DEFAULT ''::text NOT NULL,
    published timestamp with time zone DEFAULT now() NOT NULL,
    link text NOT NULL,
    CONSTRAINT c_pk_feeditem PRIMARY KEY (feed_id, guid),
    CONSTRAINT c_fk_feeditem_feed FOREIGN KEY (feed_id)
        REFERENCES okihome.t_feed (id) MATCH SIMPLE
        ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE TABLE tj_feeditem_user (
    user_id text NOT NULL,
    feed_id bigint NOT NULL,
    guid text NOT NULL,
    read boolean,
    CONSTRAINT c_pk_feeditem_user PRIMARY KEY (user_id, feed_id, guid),
    CONSTRAINT c_fk_feeditem_user_user FOREIGN KEY (user_id)
        REFERENCES okihome.t_user (id) MATCH SIMPLE
        ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE TABLE t_temporarycode (
    code text NOT NULL,
    user_id text,
    provider text,
    date time with time zone,
    CONSTRAINT c_pk_temporarycode PRIMARY KEY (code),
    CONSTRAINT c_fk_temporarycode_user FOREIGN KEY (user_id)
        REFERENCES okihome.t_user (id) MATCH SIMPLE
        ON UPDATE CASCADE ON DELETE CASCADE
);
