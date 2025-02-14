/*
 * Copyright 2017-2019 Baidu Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

#include "openrasp_sql.h"
#include "openrasp_hook.h"

POST_HOOK_FUNCTION(pg_connect, DB_CONNECTION);
POST_HOOK_FUNCTION(pg_pconnect, DB_CONNECTION);
PRE_HOOK_FUNCTION(pg_query, SQL);
PRE_HOOK_FUNCTION(pg_send_query, SQL);
PRE_HOOK_FUNCTION(pg_prepare, SQL_PREPARED);

void parse_connection_string(char *connstring, sql_connection_entry *sql_connection_p)
{
    pg_conninfo_parse(connstring,
                      [&sql_connection_p](const char *pname, const char *pval) {
                          sql_connection_p->set_name_value(pname, pval);
                      });
}

static bool init_pg_connection_entry(INTERNAL_FUNCTION_PARAMETERS, sql_connection_entry *sql_connection_p)
{
    char *host = nullptr, *port = nullptr, *options = nullptr, *tty = nullptr, *dbname = nullptr, *connstring = nullptr;
    zval *args = nullptr;
    int connect_type = 0;
    args = (zval *)safe_emalloc(ZEND_NUM_ARGS(), sizeof(zval), 0);
    if (ZEND_NUM_ARGS() < 1 || ZEND_NUM_ARGS() > 5 ||
        zend_get_parameters_array_ex(ZEND_NUM_ARGS(), args) == FAILURE)
    {
        efree(args);
        return false;
    }

    sql_connection_p->set_server("pgsql");
    if (ZEND_NUM_ARGS() == 1)
    { /* new style, using connection string */
        connstring = Z_STRVAL(args[0]);
    }
    else if (ZEND_NUM_ARGS() == 2)
    { /* Safe to add conntype_option, since 2 args was illegal */
        connstring = Z_STRVAL(args[0]);
        convert_to_long_ex(&args[1]);
        connect_type = (int)Z_LVAL(args[1]);
    }
    if (connstring)
    {
        sql_connection_p->set_connection_string(connstring);
        parse_connection_string(connstring, sql_connection_p);
    }
    efree(args);
    return true;
}

/**
 * pg_connect
 */
void post_global_pg_connect_DB_CONNECTION(OPENRASP_INTERNAL_FUNCTION_PARAMETERS)
{
    if (Z_TYPE_P(return_value) == IS_RESOURCE &&
        check_database_connection_username(INTERNAL_FUNCTION_PARAM_PASSTHRU, init_pg_connection_entry))
    {
        handle_block();
    }
}

/**
 * pg_pconnect 
 */
void post_global_pg_pconnect_DB_CONNECTION(OPENRASP_INTERNAL_FUNCTION_PARAMETERS)
{
    post_global_pg_connect_DB_CONNECTION(OPENRASP_INTERNAL_FUNCTION_PARAM_PASSTHRU);
}

/**
 * pg_query
 */
void pre_global_pg_query_SQL(OPENRASP_INTERNAL_FUNCTION_PARAMETERS)
{
    zval *pgsql_link = nullptr;
    char *query = nullptr;
    size_t query_len = 0;
    size_t argc = ZEND_NUM_ARGS();

    if (argc == 1)
    {
        if (zend_parse_parameters(ZEND_NUM_ARGS(), "s", &query, &query_len) == FAILURE)
        {
            return;
        }
    }
    else
    {
        if (zend_parse_parameters(ZEND_NUM_ARGS(), "rs", &pgsql_link, &query, &query_len) == FAILURE)
        {
            return;
        }
    }

    plugin_sql_check(query, query_len, "pgsql");
}

/**
 * pg_send_query
 */
void pre_global_pg_send_query_SQL(OPENRASP_INTERNAL_FUNCTION_PARAMETERS)
{
    pre_global_pg_query_SQL(OPENRASP_INTERNAL_FUNCTION_PARAM_PASSTHRU);
}

void pre_global_pg_prepare_SQL_PREPARED(OPENRASP_INTERNAL_FUNCTION_PARAMETERS)
{
    zval *pgsql_link = nullptr;
    char *query = nullptr;
    char *stmtname = nullptr;
    size_t query_len = 0;
    size_t stmtname_len = 0;
    size_t argc = ZEND_NUM_ARGS();

    if (argc == 2)
    {
        if (zend_parse_parameters(argc, "ss", &stmtname, &stmtname_len, &query, &query_len) == FAILURE)
        {
            return;
        }
    }
    else
    {
        if (zend_parse_parameters(argc, "rss", &pgsql_link, &stmtname, &stmtname_len, &query, &query_len) == FAILURE)
        {
            return;
        }
    }
    plugin_sql_check(query, query_len, "pgsql");
}