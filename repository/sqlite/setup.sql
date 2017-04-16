-- Copyright 2017 Simon HEGE. All rights reserved.
-- Use of this source code is governed by a MIT-style
-- license that can be found in the LICENSE file.

--
-- SQLite database setup for Okihome
--

CREATE TABLE t_user (
    id text PRIMARY KEY,
    display_name text,
    email text,
    isadmin boolean
);

CREATE TABLE t_tab (
    id integer PRIMARY KEY,
    title text,
    pos integer,
    layout text
);

CREATE TABLE tj_tabaccess (
    tab_id integer NOT NULL,
    user_id text NOT NULL,
    CONSTRAINT c_pk_tabaccess PRIMARY KEY (user_id, tab_id),
    CONSTRAINT c_fk_tab FOREIGN KEY (tab_id)
        REFERENCES t_tab (id) MATCH SIMPLE
        ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT c_fk_user FOREIGN KEY (user_id)
        REFERENCES t_user (id) MATCH SIMPLE
        ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE TABLE t_widget (
    id integer PRIMARY KEY,
    tab_id integer NOT NULL,
    type text,
    config text,
    CONSTRAINT c_fk_widget_tab FOREIGN KEY (tab_id)
        REFERENCES t_tab (id) MATCH SIMPLE
        ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE TABLE t_account (
    id integer PRIMARY KEY,
    user_id text NOT NULL,
    provider text NOT NULL,
    account_id text NOT NULL,
    token text NOT NULL,
    CONSTRAINT c_fk_account_user FOREIGN KEY (user_id)
        REFERENCES t_user (id) MATCH SIMPLE
        ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE TABLE t_emailitem (
    account_id integer NOT NULL,
    guid text NOT NULL,
    title text DEFAULT '' NOT NULL,
    read boolean DEFAULT false NOT NULL,
    published TEXT DEFAULT (date('now')) NOT NULL,
    link text DEFAULT '' NOT NULL,
    sender text DEFAULT '' NOT NULL,
    snippet text DEFAULT '' NOT NULL,
    version integer DEFAULT 0 NOT NULL,
    CONSTRAINT c_pk_emailitem PRIMARY KEY (account_id, guid),
    CONSTRAINT c_fk_emailitem_account FOREIGN KEY (account_id)
        REFERENCES t_account (id) MATCH SIMPLE
        ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE TABLE t_feed (
    id integer PRIMARY KEY,
    url text NOT NULL,
    next_retrieval TEXT DEFAULT (date('now')) NOT NULL,
    title text DEFAULT '' NOT NULL
);

CREATE TABLE t_feeditem (
    feed_id integer NOT NULL,
    guid text NOT NULL,
    title text DEFAULT '' NOT NULL,
    published TEXT DEFAULT (date('now')) NOT NULL,
    link text NOT NULL,
    CONSTRAINT c_pk_feeditem PRIMARY KEY (feed_id, guid),
    CONSTRAINT c_fk_feeditem_feed FOREIGN KEY (feed_id)
        REFERENCES t_feed (id) MATCH SIMPLE
        ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE TABLE tj_feeditem_user (
    user_id text NOT NULL,
    feed_id integer NOT NULL,
    guid text NOT NULL,
    read boolean,
    CONSTRAINT c_pk_feeditem_user PRIMARY KEY (user_id, feed_id, guid),
    CONSTRAINT c_fk_feeditem_user_user FOREIGN KEY (user_id)
        REFERENCES t_user (id) MATCH SIMPLE
        ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE TABLE t_temporarycode (
    code text PRIMARY KEY,
    user_id text,
    provider text,
    date text,
    CONSTRAINT c_fk_temporarycode_user FOREIGN KEY (user_id)
        REFERENCES t_user (id) MATCH SIMPLE
        ON UPDATE CASCADE ON DELETE CASCADE
);
