--
-- PostgreSQL database dump
--


-- Dumped from database version 16.15 (Debian 16.15-1.pgdg12+2)
-- Dumped by pg_dump version 16.15 (Debian 16.15-1.pgdg12+2)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: discourse_functions; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA discourse_functions;


--
-- Name: hstore; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS hstore WITH SCHEMA public;


--
-- Name: EXTENSION hstore; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION hstore IS 'data type for storing sets of (key, value) pairs';


--
-- Name: pg_trgm; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;


--
-- Name: EXTENSION pg_trgm; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION pg_trgm IS 'text similarity measurement and index searching based on trigrams';


--
-- Name: unaccent; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS unaccent WITH SCHEMA public;


--
-- Name: EXTENSION unaccent; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION unaccent IS 'text search dictionary that removes accents';


--
-- Name: vector; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public;


--
-- Name: EXTENSION vector; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION vector IS 'vector data type and ivfflat and hnsw access methods';


--
-- Name: ai_moderation_setting_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.ai_moderation_setting_type AS ENUM (
    'spam',
    'nsfw',
    'custom'
);


--
-- Name: hotlinked_media_status; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.hotlinked_media_status AS ENUM (
    'downloaded',
    'too_large',
    'download_failed',
    'upload_create_failed'
);


--
-- Name: raise_category_settings_require_reply_approval_readonly(); Type: FUNCTION; Schema: discourse_functions; Owner: -
--

CREATE FUNCTION discourse_functions.raise_category_settings_require_reply_approval_readonly() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
  BEGIN
    RAISE EXCEPTION 'Discourse: require_reply_approval in category_settings is readonly';
  END
$$;


--
-- Name: raise_category_settings_require_topic_approval_readonly(); Type: FUNCTION; Schema: discourse_functions; Owner: -
--

CREATE FUNCTION discourse_functions.raise_category_settings_require_topic_approval_readonly() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
  BEGIN
    RAISE EXCEPTION 'Discourse: require_topic_approval in category_settings is readonly';
  END
$$;


--
-- Name: raise_discourse_rss_polling_rss_feeds_author_readonly(); Type: FUNCTION; Schema: discourse_functions; Owner: -
--

CREATE FUNCTION discourse_functions.raise_discourse_rss_polling_rss_feeds_author_readonly() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
  BEGIN
    RAISE EXCEPTION 'Discourse: author in discourse_rss_polling_rss_feeds is readonly';
  END
$$;


--
-- Name: raise_discourse_voting_category_settings_readonly(); Type: FUNCTION; Schema: discourse_functions; Owner: -
--

CREATE FUNCTION discourse_functions.raise_discourse_voting_category_settings_readonly() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
  BEGIN
    RAISE EXCEPTION 'Discourse: discourse_voting_category_settings is read only';
  END
$$;


--
-- Name: raise_discourse_voting_topic_vote_count_readonly(); Type: FUNCTION; Schema: discourse_functions; Owner: -
--

CREATE FUNCTION discourse_functions.raise_discourse_voting_topic_vote_count_readonly() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
  BEGIN
    RAISE EXCEPTION 'Discourse: discourse_voting_topic_vote_count is read only';
  END
$$;


--
-- Name: raise_discourse_voting_votes_readonly(); Type: FUNCTION; Schema: discourse_functions; Owner: -
--

CREATE FUNCTION discourse_functions.raise_discourse_voting_votes_readonly() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
  BEGIN
    RAISE EXCEPTION 'Discourse: discourse_voting_votes is read only';
  END
$$;


--
-- Name: raise_topic_timers_topic_id_readonly(); Type: FUNCTION; Schema: discourse_functions; Owner: -
--

CREATE FUNCTION discourse_functions.raise_topic_timers_topic_id_readonly() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
  BEGIN
    RAISE EXCEPTION 'Discourse: topic_id in topic_timers is readonly';
  END
$$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: access_control_lists; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.access_control_lists (
    id bigint NOT NULL,
    target_type character varying(255) NOT NULL,
    target_id bigint NOT NULL,
    owner character varying(100) NOT NULL,
    permission character varying(100) NOT NULL,
    allowed_user_ids bigint[] DEFAULT '{}'::bigint[] NOT NULL,
    allowed_group_ids bigint[] DEFAULT '{}'::bigint[] NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: access_control_lists_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.access_control_lists_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: access_control_lists_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.access_control_lists_id_seq OWNED BY public.access_control_lists.id;


--
-- Name: ad_plugin_house_ads; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ad_plugin_house_ads (
    id bigint NOT NULL,
    name character varying NOT NULL,
    html text NOT NULL,
    visible_to_logged_in_users boolean DEFAULT true NOT NULL,
    visible_to_anons boolean DEFAULT true NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: ad_plugin_house_ads_categories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ad_plugin_house_ads_categories (
    ad_plugin_house_ad_id bigint NOT NULL,
    category_id bigint NOT NULL
);


--
-- Name: ad_plugin_house_ads_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ad_plugin_house_ads_groups (
    ad_plugin_house_ad_id bigint NOT NULL,
    group_id bigint NOT NULL
);


--
-- Name: ad_plugin_house_ads_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ad_plugin_house_ads_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ad_plugin_house_ads_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ad_plugin_house_ads_id_seq OWNED BY public.ad_plugin_house_ads.id;


--
-- Name: ad_plugin_house_ads_routes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ad_plugin_house_ads_routes (
    ad_plugin_house_ad_id bigint NOT NULL,
    route_name character varying NOT NULL
);


--
-- Name: ad_plugin_impressions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ad_plugin_impressions (
    id bigint NOT NULL,
    ad_type integer NOT NULL,
    ad_plugin_house_ad_id bigint,
    placement character varying NOT NULL,
    user_id integer,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    clicked_at timestamp(6) without time zone
);


--
-- Name: ad_plugin_impressions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ad_plugin_impressions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ad_plugin_impressions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ad_plugin_impressions_id_seq OWNED BY public.ad_plugin_impressions.id;


--
-- Name: admin_dashboard_reports; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.admin_dashboard_reports (
    id bigint NOT NULL,
    "position" integer NOT NULL,
    source character varying NOT NULL,
    identifier character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    rows integer DEFAULT 1 NOT NULL,
    cols integer DEFAULT 1 NOT NULL
);


--
-- Name: admin_dashboard_reports_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.admin_dashboard_reports_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: admin_dashboard_reports_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.admin_dashboard_reports_id_seq OWNED BY public.admin_dashboard_reports.id;


--
-- Name: admin_dashboard_sections; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.admin_dashboard_sections (
    id bigint NOT NULL,
    section_id character varying NOT NULL,
    "position" integer NOT NULL,
    visible boolean DEFAULT true NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    settings jsonb DEFAULT '{}'::jsonb NOT NULL
);


--
-- Name: admin_dashboard_sections_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.admin_dashboard_sections_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: admin_dashboard_sections_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.admin_dashboard_sections_id_seq OWNED BY public.admin_dashboard_sections.id;


--
-- Name: admin_notices; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.admin_notices (
    id bigint NOT NULL,
    subject integer NOT NULL,
    priority integer NOT NULL,
    identifier character varying NOT NULL,
    details json DEFAULT '{}'::json NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: admin_notices_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.admin_notices_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: admin_notices_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.admin_notices_id_seq OWNED BY public.admin_notices.id;


--
-- Name: ai_agent_mcp_servers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_agent_mcp_servers (
    id bigint NOT NULL,
    ai_agent_id bigint NOT NULL,
    ai_mcp_server_id bigint NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    selected_tool_names jsonb
);


--
-- Name: ai_agent_mcp_servers_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ai_agent_mcp_servers_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ai_agent_mcp_servers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ai_agent_mcp_servers_id_seq OWNED BY public.ai_agent_mcp_servers.id;


--
-- Name: ai_agents; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_agents (
    id bigint NOT NULL,
    name character varying(100) NOT NULL,
    description character varying(2000) NOT NULL,
    system_prompt character varying(10000000) NOT NULL,
    allowed_group_ids integer[] DEFAULT '{}'::integer[] NOT NULL,
    created_by_id integer,
    enabled boolean DEFAULT true NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    system boolean DEFAULT false NOT NULL,
    priority boolean DEFAULT false NOT NULL,
    temperature double precision,
    top_p double precision,
    user_id integer,
    vision_enabled boolean DEFAULT false NOT NULL,
    vision_max_pixels integer DEFAULT 1048576 NOT NULL,
    rag_chunk_tokens integer DEFAULT 374 NOT NULL,
    rag_chunk_overlap_tokens integer DEFAULT 10 NOT NULL,
    rag_conversation_chunks integer DEFAULT 10 NOT NULL,
    tools json DEFAULT '[]'::json NOT NULL,
    forced_tool_count integer DEFAULT '-1'::integer NOT NULL,
    allow_chat_channel_mentions boolean DEFAULT false NOT NULL,
    allow_chat_direct_messages boolean DEFAULT false NOT NULL,
    allow_topic_mentions boolean DEFAULT false NOT NULL,
    allow_personal_messages boolean DEFAULT true NOT NULL,
    force_default_llm boolean DEFAULT false NOT NULL,
    rag_llm_model_id bigint,
    default_llm_id bigint,
    question_consolidator_llm_id bigint,
    response_format jsonb,
    examples jsonb,
    show_thinking boolean DEFAULT true NOT NULL,
    max_turn_tokens integer,
    compression_threshold integer DEFAULT 80 NOT NULL,
    require_approval boolean DEFAULT false NOT NULL,
    thinking_effort character varying,
    subagent_ids bigint[] DEFAULT '{}'::bigint[] NOT NULL
);


--
-- Name: ai_agents_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ai_agents_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ai_agents_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ai_agents_id_seq OWNED BY public.ai_agents.id;


--
-- Name: ai_api_audit_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_api_audit_logs (
    id bigint NOT NULL,
    provider_id integer NOT NULL,
    user_id integer,
    request_tokens integer,
    response_tokens integer,
    raw_request_payload character varying,
    raw_response_payload character varying,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    topic_id integer,
    post_id integer,
    feature_name character varying(255),
    language_model character varying(255),
    feature_context jsonb,
    duration_msecs integer,
    cache_write_tokens integer,
    cache_read_tokens integer,
    llm_id bigint,
    response_status integer,
    request_attempts jsonb,
    estimated_cost numeric(20,10),
    time_to_first_token_msecs integer
);


--
-- Name: ai_api_audit_logs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ai_api_audit_logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ai_api_audit_logs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ai_api_audit_logs_id_seq OWNED BY public.ai_api_audit_logs.id;


--
-- Name: ai_api_request_stats; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_api_request_stats (
    id bigint NOT NULL,
    bucket_date timestamp(6) without time zone NOT NULL,
    user_id bigint,
    provider_id integer NOT NULL,
    llm_id bigint,
    language_model character varying(255),
    feature_name character varying(255),
    request_tokens integer DEFAULT 0 NOT NULL,
    response_tokens integer DEFAULT 0 NOT NULL,
    cache_read_tokens integer DEFAULT 0 NOT NULL,
    cache_write_tokens integer DEFAULT 0 NOT NULL,
    usage_count integer DEFAULT 1 NOT NULL,
    rolled_up boolean DEFAULT false NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    estimated_cost numeric(20,10)
);


--
-- Name: ai_api_request_stats_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ai_api_request_stats_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ai_api_request_stats_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ai_api_request_stats_id_seq OWNED BY public.ai_api_request_stats.id;


--
-- Name: ai_artifact_key_values; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_artifact_key_values (
    id bigint NOT NULL,
    ai_artifact_id bigint NOT NULL,
    user_id integer NOT NULL,
    key character varying(50) NOT NULL,
    value character varying(20000) NOT NULL,
    public boolean DEFAULT false NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: ai_artifact_key_values_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ai_artifact_key_values_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ai_artifact_key_values_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ai_artifact_key_values_id_seq OWNED BY public.ai_artifact_key_values.id;


--
-- Name: ai_artifact_versions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_artifact_versions (
    id bigint NOT NULL,
    ai_artifact_id bigint NOT NULL,
    version_number integer NOT NULL,
    html character varying(65535),
    css character varying(65535),
    js character varying(65535),
    metadata jsonb,
    change_description character varying,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: ai_artifact_versions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ai_artifact_versions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ai_artifact_versions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ai_artifact_versions_id_seq OWNED BY public.ai_artifact_versions.id;


--
-- Name: ai_artifacts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_artifacts (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    post_id integer NOT NULL,
    name character varying(255) NOT NULL,
    html character varying(65535),
    css character varying(65535),
    js character varying(65535),
    metadata jsonb,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: ai_artifacts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ai_artifacts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ai_artifacts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ai_artifacts_id_seq OWNED BY public.ai_artifacts.id;


--
-- Name: ai_document_fragments_embeddings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_document_fragments_embeddings (
    rag_document_fragment_id bigint NOT NULL,
    model_id bigint NOT NULL,
    model_version integer NOT NULL,
    strategy_id integer NOT NULL,
    strategy_version integer NOT NULL,
    digest text NOT NULL,
    embeddings public.halfvec NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: ai_mcp_oauth_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_mcp_oauth_tokens (
    id bigint NOT NULL,
    ai_mcp_server_id bigint NOT NULL,
    access_token text,
    refresh_token text,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: ai_mcp_oauth_tokens_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ai_mcp_oauth_tokens_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ai_mcp_oauth_tokens_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ai_mcp_oauth_tokens_id_seq OWNED BY public.ai_mcp_oauth_tokens.id;


--
-- Name: ai_mcp_servers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_mcp_servers (
    id bigint NOT NULL,
    name character varying(100) NOT NULL,
    description character varying(1000) NOT NULL,
    url character varying(1000) NOT NULL,
    ai_secret_id bigint,
    auth_header character varying(100) DEFAULT 'Authorization'::character varying NOT NULL,
    auth_scheme character varying(100) DEFAULT 'Bearer'::character varying NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    timeout_seconds integer DEFAULT 30 NOT NULL,
    last_health_status character varying(50),
    last_health_error character varying(1000),
    last_checked_at timestamp(6) without time zone,
    last_tools_synced_at timestamp(6) without time zone,
    server_capabilities jsonb DEFAULT '{}'::jsonb NOT NULL,
    protocol_version character varying(100),
    created_by_id integer,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    auth_type character varying(50) DEFAULT 'header_secret'::character varying NOT NULL,
    oauth_client_registration character varying(50) DEFAULT 'client_metadata_document'::character varying,
    oauth_client_id character varying(1000),
    oauth_client_secret_ai_secret_id bigint,
    oauth_scopes character varying(2000),
    oauth_granted_scopes character varying(2000),
    oauth_token_type character varying(100),
    oauth_access_token_expires_at timestamp(6) without time zone,
    oauth_authorization_endpoint character varying(1000),
    oauth_token_endpoint character varying(1000),
    oauth_revocation_endpoint character varying(1000),
    oauth_issuer character varying(1000),
    oauth_resource_metadata_url character varying(1000),
    oauth_status character varying(50) DEFAULT 'disconnected'::character varying NOT NULL,
    oauth_last_error character varying(1000),
    oauth_last_authorized_at timestamp(6) without time zone,
    oauth_last_refreshed_at timestamp(6) without time zone,
    oauth_registration_endpoint character varying(1000),
    oauth_authorization_params jsonb DEFAULT '{}'::jsonb NOT NULL,
    oauth_token_params jsonb DEFAULT '{}'::jsonb NOT NULL,
    oauth_require_refresh_token boolean DEFAULT false NOT NULL,
    oauth_token_endpoint_auth_methods_supported jsonb DEFAULT '[]'::jsonb NOT NULL
);


--
-- Name: ai_mcp_servers_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ai_mcp_servers_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ai_mcp_servers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ai_mcp_servers_id_seq OWNED BY public.ai_mcp_servers.id;


--
-- Name: ai_moderation_settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_moderation_settings (
    id bigint NOT NULL,
    setting_type public.ai_moderation_setting_type NOT NULL,
    data jsonb DEFAULT '{}'::jsonb,
    llm_model_id bigint NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    ai_agent_id bigint DEFAULT '-31'::integer NOT NULL
);


--
-- Name: ai_moderation_settings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ai_moderation_settings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ai_moderation_settings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ai_moderation_settings_id_seq OWNED BY public.ai_moderation_settings.id;


--
-- Name: ai_post_image_captions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_post_image_captions (
    id bigint NOT NULL,
    post_id integer NOT NULL,
    upload_id integer NOT NULL,
    base62_sha1 character varying(27) NOT NULL,
    locale character varying(20) NOT NULL,
    description text,
    attempts integer DEFAULT 0 NOT NULL,
    last_attempted_at timestamp(6) without time zone,
    last_error text,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: ai_post_image_captions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ai_post_image_captions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ai_post_image_captions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ai_post_image_captions_id_seq OWNED BY public.ai_post_image_captions.id;


--
-- Name: ai_posts_embeddings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_posts_embeddings (
    post_id bigint NOT NULL,
    model_id bigint NOT NULL,
    model_version integer NOT NULL,
    strategy_id integer NOT NULL,
    strategy_version integer NOT NULL,
    digest text NOT NULL,
    embeddings public.halfvec NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: ai_secrets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_secrets (
    id bigint NOT NULL,
    name character varying(100) NOT NULL,
    secret character varying(10000) NOT NULL,
    created_by_id integer,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: ai_secrets_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ai_secrets_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ai_secrets_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ai_secrets_id_seq OWNED BY public.ai_secrets.id;


--
-- Name: ai_spam_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_spam_logs (
    id bigint NOT NULL,
    post_id bigint NOT NULL,
    llm_model_id bigint NOT NULL,
    ai_api_audit_log_id bigint,
    reviewable_id bigint,
    is_spam boolean NOT NULL,
    payload character varying(20000) DEFAULT ''::character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    error character varying(3000),
    reason text
);


--
-- Name: ai_spam_logs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ai_spam_logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ai_spam_logs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ai_spam_logs_id_seq OWNED BY public.ai_spam_logs.id;


--
-- Name: ai_summaries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_summaries (
    id bigint NOT NULL,
    target_id integer NOT NULL,
    target_type character varying NOT NULL,
    summarized_text character varying NOT NULL,
    original_content_sha character varying NOT NULL,
    algorithm character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    summary_type integer DEFAULT 0 NOT NULL,
    origin integer,
    highest_target_number integer DEFAULT 1 NOT NULL,
    locale character varying(20)
);


--
-- Name: ai_summaries_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ai_summaries_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ai_summaries_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ai_summaries_id_seq OWNED BY public.ai_summaries.id;


--
-- Name: ai_tool_actions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_tool_actions (
    id bigint NOT NULL,
    tool_name character varying NOT NULL,
    tool_parameters jsonb DEFAULT '{}'::jsonb NOT NULL,
    ai_agent_id bigint NOT NULL,
    bot_user_id integer NOT NULL,
    post_id integer,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: ai_tool_actions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ai_tool_actions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ai_tool_actions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ai_tool_actions_id_seq OWNED BY public.ai_tool_actions.id;


--
-- Name: ai_tool_secret_bindings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_tool_secret_bindings (
    id bigint NOT NULL,
    ai_tool_id bigint NOT NULL,
    alias character varying(100) NOT NULL,
    ai_secret_id bigint NOT NULL,
    created_by_id integer,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: ai_tool_secret_bindings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ai_tool_secret_bindings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ai_tool_secret_bindings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ai_tool_secret_bindings_id_seq OWNED BY public.ai_tool_secret_bindings.id;


--
-- Name: ai_tools; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_tools (
    id bigint NOT NULL,
    name character varying NOT NULL,
    description character varying NOT NULL,
    summary character varying NOT NULL,
    parameters jsonb DEFAULT '{}'::jsonb NOT NULL,
    script text NOT NULL,
    created_by_id integer NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    rag_chunk_tokens integer DEFAULT 374 NOT NULL,
    rag_chunk_overlap_tokens integer DEFAULT 10 NOT NULL,
    tool_name character varying(100) DEFAULT ''::character varying NOT NULL,
    rag_llm_model_id bigint,
    is_image_generation_tool boolean DEFAULT false NOT NULL,
    secret_contracts jsonb DEFAULT '[]'::jsonb NOT NULL
);


--
-- Name: ai_tools_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ai_tools_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ai_tools_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ai_tools_id_seq OWNED BY public.ai_tools.id;


--
-- Name: ai_topics_embeddings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_topics_embeddings (
    topic_id bigint NOT NULL,
    model_id bigint NOT NULL,
    model_version integer NOT NULL,
    strategy_id integer NOT NULL,
    strategy_version integer NOT NULL,
    digest text NOT NULL,
    embeddings public.halfvec NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: allowed_pm_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.allowed_pm_users (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    allowed_pm_user_id integer NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: allowed_pm_users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.allowed_pm_users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: allowed_pm_users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.allowed_pm_users_id_seq OWNED BY public.allowed_pm_users.id;


--
-- Name: anonymous_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.anonymous_users (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    master_user_id integer NOT NULL,
    active boolean NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: anonymous_users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.anonymous_users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: anonymous_users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.anonymous_users_id_seq OWNED BY public.anonymous_users.id;


--
-- Name: api_key_scopes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.api_key_scopes (
    id bigint NOT NULL,
    api_key_id integer NOT NULL,
    resource character varying NOT NULL,
    action character varying NOT NULL,
    allowed_parameters json,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: api_key_scopes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.api_key_scopes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: api_key_scopes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.api_key_scopes_id_seq OWNED BY public.api_key_scopes.id;


--
-- Name: api_keys; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.api_keys (
    id integer NOT NULL,
    user_id integer,
    created_by_id integer,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    allowed_ips inet[],
    hidden boolean DEFAULT false NOT NULL,
    last_used_at timestamp without time zone,
    revoked_at timestamp without time zone,
    description text,
    key_hash character varying NOT NULL,
    truncated_key character varying NOT NULL,
    scope_mode integer
);


--
-- Name: api_keys_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.api_keys_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: api_keys_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.api_keys_id_seq OWNED BY public.api_keys.id;


--
-- Name: application_requests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.application_requests (
    id integer NOT NULL,
    date date NOT NULL,
    req_type integer NOT NULL,
    count integer DEFAULT 0 NOT NULL
);


--
-- Name: application_requests_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.application_requests_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: application_requests_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.application_requests_id_seq OWNED BY public.application_requests.id;


--
-- Name: ar_internal_metadata; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ar_internal_metadata (
    key character varying NOT NULL,
    value character varying,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: ask_ai_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ask_ai_logs (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    query text NOT NULL,
    keyword_query text,
    semantic_query text,
    query_locale character varying,
    candidate_post_ids bigint[] DEFAULT '{}'::bigint[] NOT NULL,
    source_post_ids bigint[] DEFAULT '{}'::bigint[] NOT NULL,
    answer_title text,
    answer text,
    suggested_follow_up text,
    ask_outcome integer,
    failure_stage integer,
    asked_at timestamp(6) without time zone NOT NULL,
    time_to_first_answer_ms integer,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: ask_ai_logs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ask_ai_logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ask_ai_logs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ask_ai_logs_id_seq OWNED BY public.ask_ai_logs.id;


--
-- Name: assignments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.assignments (
    id bigint NOT NULL,
    topic_id integer NOT NULL,
    assigned_to_id integer NOT NULL,
    assigned_by_user_id integer NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    assigned_to_type character varying NOT NULL,
    target_id integer NOT NULL,
    target_type character varying NOT NULL,
    active boolean DEFAULT true,
    note character varying,
    status text
);


--
-- Name: assignments_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.assignments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: assignments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.assignments_id_seq OWNED BY public.assignments.id;


--
-- Name: associated_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.associated_groups (
    id bigint NOT NULL,
    name character varying NOT NULL,
    provider_name character varying NOT NULL,
    provider_id character varying NOT NULL,
    last_used timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: associated_groups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.associated_groups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: associated_groups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.associated_groups_id_seq OWNED BY public.associated_groups.id;


--
-- Name: backup_draft_posts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.backup_draft_posts (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    post_id integer NOT NULL,
    key character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: backup_draft_posts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.backup_draft_posts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: backup_draft_posts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.backup_draft_posts_id_seq OWNED BY public.backup_draft_posts.id;


--
-- Name: backup_draft_topics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.backup_draft_topics (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    topic_id integer NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: backup_draft_topics_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.backup_draft_topics_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: backup_draft_topics_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.backup_draft_topics_id_seq OWNED BY public.backup_draft_topics.id;


--
-- Name: backup_metadata; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.backup_metadata (
    id bigint NOT NULL,
    name character varying NOT NULL,
    value character varying
);


--
-- Name: backup_metadata_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.backup_metadata_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: backup_metadata_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.backup_metadata_id_seq OWNED BY public.backup_metadata.id;


--
-- Name: badge_groupings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.badge_groupings (
    id integer NOT NULL,
    name character varying NOT NULL,
    description text,
    "position" integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: badge_groupings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.badge_groupings_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: badge_groupings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.badge_groupings_id_seq OWNED BY public.badge_groupings.id;


--
-- Name: categories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.categories (
    id integer NOT NULL,
    name character varying(50) NOT NULL,
    color character varying(6) DEFAULT '0088CC'::character varying NOT NULL,
    topic_id integer,
    topic_count integer DEFAULT 0 NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    user_id integer NOT NULL,
    topics_year integer DEFAULT 0,
    topics_month integer DEFAULT 0,
    topics_week integer DEFAULT 0,
    slug character varying NOT NULL,
    description text,
    text_color character varying(6) DEFAULT 'FFFFFF'::character varying NOT NULL,
    read_restricted boolean DEFAULT false NOT NULL,
    auto_close_hours double precision,
    post_count integer DEFAULT 0 NOT NULL,
    latest_post_id integer,
    latest_topic_id integer,
    "position" integer,
    parent_category_id integer,
    posts_year integer DEFAULT 0,
    posts_month integer DEFAULT 0,
    posts_week integer DEFAULT 0,
    email_in character varying,
    email_in_allow_strangers boolean DEFAULT false,
    topics_day integer DEFAULT 0,
    posts_day integer DEFAULT 0,
    allow_badges boolean DEFAULT true NOT NULL,
    name_lower character varying(50) NOT NULL,
    auto_close_based_on_last_post boolean DEFAULT false,
    topic_template text,
    contains_messages boolean,
    sort_order character varying,
    sort_ascending boolean,
    uploaded_logo_id integer,
    uploaded_background_id integer,
    topic_featured_link_allowed boolean DEFAULT true,
    all_topics_wiki boolean DEFAULT false NOT NULL,
    show_subcategory_list boolean DEFAULT false,
    num_featured_topics integer DEFAULT 3,
    default_view character varying(50),
    subcategory_list_style character varying(50) DEFAULT 'rows_with_featured_topics'::character varying,
    default_top_period character varying(20) DEFAULT 'all'::character varying,
    mailinglist_mirror boolean DEFAULT false NOT NULL,
    minimum_required_tags integer DEFAULT 0 NOT NULL,
    navigate_to_first_post_after_read boolean DEFAULT false NOT NULL,
    search_priority integer DEFAULT 0,
    allow_global_tags boolean DEFAULT false NOT NULL,
    reviewable_by_group_id integer,
    read_only_banner character varying,
    default_list_filter character varying(20) DEFAULT 'all'::character varying,
    allow_unlimited_owner_edits_on_first_post boolean DEFAULT false NOT NULL,
    default_slow_mode_seconds integer,
    uploaded_logo_dark_id integer,
    uploaded_background_dark_id integer,
    style_type integer DEFAULT 0 NOT NULL,
    emoji character varying,
    icon character varying,
    locale character varying(20),
    topic_title_placeholder character varying
);


--
-- Name: posts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.posts (
    id integer NOT NULL,
    user_id integer,
    topic_id integer NOT NULL,
    post_number integer NOT NULL,
    raw text NOT NULL,
    cooked text NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    reply_to_post_number integer,
    reply_count integer DEFAULT 0 NOT NULL,
    quote_count integer DEFAULT 0 NOT NULL,
    deleted_at timestamp without time zone,
    off_topic_count integer DEFAULT 0 NOT NULL,
    like_count integer DEFAULT 0 NOT NULL,
    incoming_link_count integer DEFAULT 0 NOT NULL,
    bookmark_count integer DEFAULT 0 NOT NULL,
    score double precision,
    reads integer DEFAULT 0 NOT NULL,
    post_type integer DEFAULT 1 NOT NULL,
    sort_order integer,
    last_editor_id integer,
    hidden boolean DEFAULT false NOT NULL,
    hidden_reason_id integer,
    notify_moderators_count integer DEFAULT 0 NOT NULL,
    spam_count integer DEFAULT 0 NOT NULL,
    illegal_count integer DEFAULT 0 NOT NULL,
    inappropriate_count integer DEFAULT 0 NOT NULL,
    last_version_at timestamp without time zone NOT NULL,
    user_deleted boolean DEFAULT false NOT NULL,
    reply_to_user_id integer,
    percent_rank double precision DEFAULT 1.0,
    notify_user_count integer DEFAULT 0 NOT NULL,
    like_score integer DEFAULT 0 NOT NULL,
    deleted_by_id integer,
    edit_reason character varying,
    word_count integer,
    version integer DEFAULT 1 NOT NULL,
    cook_method integer DEFAULT 1 NOT NULL,
    wiki boolean DEFAULT false NOT NULL,
    baked_at timestamp without time zone,
    baked_version integer,
    hidden_at timestamp without time zone,
    self_edits integer DEFAULT 0 NOT NULL,
    reply_quoted boolean DEFAULT false NOT NULL,
    via_email boolean DEFAULT false NOT NULL,
    raw_email text,
    public_version integer DEFAULT 1 NOT NULL,
    action_code character varying,
    locked_by_id integer,
    image_upload_id bigint,
    qa_vote_count integer DEFAULT 0,
    outbound_message_id character varying,
    locale character varying(20)
);


--
-- Name: TABLE posts; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.posts IS 'If you want to query public posts only, use the badge_posts view.';


--
-- Name: COLUMN posts.post_number; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.posts.post_number IS 'The position of this post in the topic. The pair (topic_id, post_number) forms a natural key on the posts table.';


--
-- Name: COLUMN posts.raw; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.posts.raw IS 'The raw Markdown that the user entered into the composer.';


--
-- Name: COLUMN posts.cooked; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.posts.cooked IS 'The processed HTML that is presented in a topic.';


--
-- Name: COLUMN posts.reply_to_post_number; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.posts.reply_to_post_number IS 'If this post is a reply to another, this column is the post_number of the post it''s replying to. [FKEY posts.topic_id, posts.post_number]';


--
-- Name: COLUMN posts.reply_quoted; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.posts.reply_quoted IS 'This column is true if the post contains a quote-reply, which causes the in-reply-to indicator to be absent.';


--
-- Name: topics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.topics (
    id integer NOT NULL,
    title character varying NOT NULL,
    last_posted_at timestamp without time zone,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    views integer DEFAULT 0 NOT NULL,
    posts_count integer DEFAULT 0 NOT NULL,
    user_id integer,
    last_post_user_id integer NOT NULL,
    reply_count integer DEFAULT 0 NOT NULL,
    featured_user1_id integer,
    featured_user2_id integer,
    featured_user3_id integer,
    deleted_at timestamp without time zone,
    highest_post_number integer DEFAULT 0 NOT NULL,
    like_count integer DEFAULT 0 NOT NULL,
    incoming_link_count integer DEFAULT 0 NOT NULL,
    category_id integer,
    visible boolean DEFAULT true NOT NULL,
    moderator_posts_count integer DEFAULT 0 NOT NULL,
    closed boolean DEFAULT false NOT NULL,
    archived boolean DEFAULT false NOT NULL,
    bumped_at timestamp without time zone NOT NULL,
    has_summary boolean DEFAULT false NOT NULL,
    archetype character varying DEFAULT 'regular'::character varying NOT NULL,
    featured_user4_id integer,
    notify_moderators_count integer DEFAULT 0 NOT NULL,
    spam_count integer DEFAULT 0 NOT NULL,
    pinned_at timestamp without time zone,
    score double precision,
    percent_rank double precision DEFAULT 1.0 NOT NULL,
    subtype character varying,
    slug character varying,
    deleted_by_id integer,
    participant_count integer DEFAULT 1,
    word_count integer,
    excerpt character varying,
    pinned_globally boolean DEFAULT false NOT NULL,
    pinned_until timestamp without time zone,
    fancy_title character varying,
    highest_staff_post_number integer DEFAULT 0 NOT NULL,
    featured_link character varying,
    reviewable_score double precision DEFAULT 0.0 NOT NULL,
    image_upload_id bigint,
    slow_mode_seconds integer DEFAULT 0 NOT NULL,
    bannered_until timestamp without time zone,
    external_id character varying,
    visibility_reason_id integer,
    locale character varying(20),
    og_image_upload_id bigint,
    CONSTRAINT has_category_id CHECK (((category_id IS NOT NULL) OR ((archetype)::text <> 'regular'::text))),
    CONSTRAINT pm_has_no_category CHECK (((category_id IS NULL) OR ((archetype)::text <> 'private_message'::text)))
);


--
-- Name: TABLE topics; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.topics IS 'To query public topics only: SELECT ... FROM topics t LEFT INNER JOIN categories c ON (t.category_id = c.id AND c.read_restricted = false)';


--
-- Name: badge_posts; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.badge_posts AS
 SELECT p.id,
    p.user_id,
    p.topic_id,
    p.post_number,
    p.raw,
    p.cooked,
    p.created_at,
    p.updated_at,
    p.reply_to_post_number,
    p.reply_count,
    p.quote_count,
    p.deleted_at,
    p.off_topic_count,
    p.like_count,
    p.incoming_link_count,
    p.bookmark_count,
    p.score,
    p.reads,
    p.post_type,
    p.sort_order,
    p.last_editor_id,
    p.hidden,
    p.hidden_reason_id,
    p.notify_moderators_count,
    p.spam_count,
    p.illegal_count,
    p.inappropriate_count,
    p.last_version_at,
    p.user_deleted,
    p.reply_to_user_id,
    p.percent_rank,
    p.notify_user_count,
    p.like_score,
    p.deleted_by_id,
    p.edit_reason,
    p.word_count,
    p.version,
    p.cook_method,
    p.wiki,
    p.baked_at,
    p.baked_version,
    p.hidden_at,
    p.self_edits,
    p.reply_quoted,
    p.via_email,
    p.raw_email,
    p.public_version,
    p.action_code,
    p.locked_by_id,
    p.image_upload_id
   FROM ((public.posts p
     JOIN public.topics t ON ((t.id = p.topic_id)))
     JOIN public.categories c ON ((c.id = t.category_id)))
  WHERE (c.allow_badges AND (p.deleted_at IS NULL) AND (t.deleted_at IS NULL) AND (NOT c.read_restricted) AND t.visible AND (p.post_type = ANY (ARRAY[1, 2, 3])));


--
-- Name: badge_types; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.badge_types (
    id integer NOT NULL,
    name character varying NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: badge_types_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.badge_types_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: badge_types_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.badge_types_id_seq OWNED BY public.badge_types.id;


--
-- Name: badges; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.badges (
    id integer NOT NULL,
    name character varying NOT NULL,
    description text,
    badge_type_id integer NOT NULL,
    grant_count integer DEFAULT 0 NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    allow_title boolean DEFAULT false NOT NULL,
    multiple_grant boolean DEFAULT false NOT NULL,
    icon character varying DEFAULT 'certificate'::character varying,
    listable boolean DEFAULT true,
    target_posts boolean DEFAULT false,
    query text,
    enabled boolean DEFAULT true NOT NULL,
    auto_revoke boolean DEFAULT true NOT NULL,
    badge_grouping_id integer DEFAULT 5 NOT NULL,
    trigger integer,
    show_posts boolean DEFAULT false NOT NULL,
    system boolean DEFAULT false NOT NULL,
    long_description text,
    image_upload_id integer,
    show_in_post_header boolean DEFAULT false NOT NULL
);


--
-- Name: badges_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.badges_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: badges_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.badges_id_seq OWNED BY public.badges.id;


--
-- Name: bookmarks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.bookmarks (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    name character varying(100),
    reminder_at timestamp without time zone,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    reminder_last_sent_at timestamp without time zone,
    reminder_set_at timestamp without time zone,
    auto_delete_preference integer DEFAULT 0 NOT NULL,
    pinned boolean DEFAULT false,
    bookmarkable_id bigint,
    bookmarkable_type character varying
);


--
-- Name: bookmarks_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.bookmarks_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: bookmarks_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.bookmarks_id_seq OWNED BY public.bookmarks.id;


--
-- Name: browser_pageview_country_daily_rollups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.browser_pageview_country_daily_rollups (
    id bigint NOT NULL,
    date date NOT NULL,
    country_code character varying(2),
    count bigint NOT NULL,
    logged_in_count bigint NOT NULL,
    likely_crawler_count bigint DEFAULT 0 NOT NULL,
    likely_crawler_logged_in_count bigint DEFAULT 0 NOT NULL
);


--
-- Name: browser_pageview_country_daily_rollups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.browser_pageview_country_daily_rollups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: browser_pageview_country_daily_rollups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.browser_pageview_country_daily_rollups_id_seq OWNED BY public.browser_pageview_country_daily_rollups.id;


--
-- Name: browser_pageview_crawler_daily_rollups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.browser_pageview_crawler_daily_rollups (
    id bigint NOT NULL,
    date date NOT NULL,
    logged_in boolean NOT NULL,
    count bigint NOT NULL
);


--
-- Name: browser_pageview_crawler_daily_rollups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.browser_pageview_crawler_daily_rollups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: browser_pageview_crawler_daily_rollups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.browser_pageview_crawler_daily_rollups_id_seq OWNED BY public.browser_pageview_crawler_daily_rollups.id;


--
-- Name: browser_pageview_entry_url_daily_rollups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.browser_pageview_entry_url_daily_rollups (
    id bigint NOT NULL,
    date date NOT NULL,
    entry_url character varying(2000) NOT NULL,
    count bigint NOT NULL,
    logged_in_count bigint NOT NULL,
    likely_crawler_count bigint DEFAULT 0 NOT NULL,
    likely_crawler_logged_in_count bigint DEFAULT 0 NOT NULL
);


--
-- Name: browser_pageview_entry_url_daily_rollups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.browser_pageview_entry_url_daily_rollups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: browser_pageview_entry_url_daily_rollups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.browser_pageview_entry_url_daily_rollups_id_seq OWNED BY public.browser_pageview_entry_url_daily_rollups.id;


--
-- Name: browser_pageview_event_scores; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.browser_pageview_event_scores (
    id bigint NOT NULL,
    event_id bigint NOT NULL,
    automation_ua_score smallint DEFAULT 0 NOT NULL,
    known_asn_score smallint DEFAULT 0 NOT NULL,
    velocity_score smallint DEFAULT 0 NOT NULL,
    churn_score smallint DEFAULT 0 NOT NULL,
    rapid_nav_score smallint DEFAULT 0 NOT NULL,
    referrer_score smallint DEFAULT 0 NOT NULL,
    engagement_score smallint DEFAULT 0 NOT NULL,
    ip_rotation_score smallint DEFAULT 0 NOT NULL,
    datacenter_asn_score smallint DEFAULT 0 NOT NULL,
    single_request_no_referrer_score smallint DEFAULT 0 NOT NULL,
    stale_browser_score smallint DEFAULT 0 NOT NULL
);


--
-- Name: browser_pageview_event_scores_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.browser_pageview_event_scores_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: browser_pageview_event_scores_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.browser_pageview_event_scores_id_seq OWNED BY public.browser_pageview_event_scores.id;


--
-- Name: browser_pageview_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.browser_pageview_events (
    id bigint NOT NULL,
    url character varying(2000) NOT NULL,
    ip_address inet NOT NULL,
    referrer character varying(2000),
    user_agent character varying(1000) NOT NULL,
    session_id character varying(32) NOT NULL,
    topic_id integer,
    user_id integer,
    country_code character varying(2),
    created_at timestamp without time zone NOT NULL,
    asn integer,
    score integer,
    normalized_referrer character varying(2000),
    normalized_referrer_version smallint,
    source smallint DEFAULT 1 NOT NULL,
    normalized_url character varying(2000),
    normalized_url_version integer,
    browser smallint,
    language character varying(255),
    normalized_language character varying
);


--
-- Name: browser_pageview_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.browser_pageview_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: browser_pageview_events_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.browser_pageview_events_id_seq OWNED BY public.browser_pageview_events.id;


--
-- Name: browser_pageview_referrer_daily_rollups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.browser_pageview_referrer_daily_rollups (
    id bigint NOT NULL,
    date date NOT NULL,
    normalized_referrer character varying(2000),
    count bigint NOT NULL,
    logged_in_count bigint NOT NULL,
    likely_crawler_count bigint DEFAULT 0 NOT NULL,
    likely_crawler_logged_in_count bigint DEFAULT 0 NOT NULL
);


--
-- Name: browser_pageview_referrer_daily_rollups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.browser_pageview_referrer_daily_rollups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: browser_pageview_referrer_daily_rollups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.browser_pageview_referrer_daily_rollups_id_seq OWNED BY public.browser_pageview_referrer_daily_rollups.id;


--
-- Name: browser_pageview_session_engagement_daily_rollups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.browser_pageview_session_engagement_daily_rollups (
    id bigint NOT NULL,
    date date NOT NULL,
    logged_in boolean NOT NULL,
    sessions bigint NOT NULL,
    bounced bigint NOT NULL,
    engaged_seconds_total bigint NOT NULL,
    likely_crawler_sessions bigint DEFAULT 0 NOT NULL,
    likely_crawler_bounced bigint DEFAULT 0 NOT NULL,
    likely_crawler_engaged_seconds_total bigint DEFAULT 0 NOT NULL
);


--
-- Name: browser_pageview_session_engagement_daily_rollups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.browser_pageview_session_engagement_daily_rollups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: browser_pageview_session_engagement_daily_rollups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.browser_pageview_session_engagement_daily_rollups_id_seq OWNED BY public.browser_pageview_session_engagement_daily_rollups.id;


--
-- Name: browser_pageview_session_engagements; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.browser_pageview_session_engagements (
    id bigint NOT NULL,
    session_id character varying(32) NOT NULL,
    mouse_move_events integer DEFAULT 0 NOT NULL,
    click_events integer DEFAULT 0 NOT NULL,
    key_events integer DEFAULT 0 NOT NULL,
    scroll_events integer DEFAULT 0 NOT NULL,
    touch_events integer DEFAULT 0 NOT NULL,
    back_forward_events integer DEFAULT 0 NOT NULL,
    engaged_seconds integer DEFAULT 0 NOT NULL,
    time_to_first_interaction_ms integer,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: browser_pageview_session_engagements_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.browser_pageview_session_engagements_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: browser_pageview_session_engagements_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.browser_pageview_session_engagements_id_seq OWNED BY public.browser_pageview_session_engagements.id;


--
-- Name: calendar_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.calendar_events (
    id bigint NOT NULL,
    topic_id integer NOT NULL,
    post_id integer,
    post_number integer,
    user_id integer,
    username character varying,
    description character varying,
    start_date timestamp without time zone NOT NULL,
    end_date timestamp without time zone,
    recurrence character varying,
    region character varying,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    timezone character varying
);


--
-- Name: calendar_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.calendar_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: calendar_events_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.calendar_events_id_seq OWNED BY public.calendar_events.id;


--
-- Name: categories_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.categories_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: categories_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.categories_id_seq OWNED BY public.categories.id;


--
-- Name: categories_web_hooks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.categories_web_hooks (
    web_hook_id integer NOT NULL,
    category_id integer NOT NULL
);


--
-- Name: category_activity_daily_rollups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.category_activity_daily_rollups (
    id bigint NOT NULL,
    date date NOT NULL,
    category_id integer NOT NULL,
    topics integer DEFAULT 0 NOT NULL,
    posts integer DEFAULT 0 NOT NULL,
    page_views bigint DEFAULT 0 NOT NULL,
    likely_crawler_page_views bigint DEFAULT 0 NOT NULL
);


--
-- Name: category_activity_daily_rollups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.category_activity_daily_rollups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: category_activity_daily_rollups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.category_activity_daily_rollups_id_seq OWNED BY public.category_activity_daily_rollups.id;


--
-- Name: category_custom_fields; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.category_custom_fields (
    id integer NOT NULL,
    category_id integer NOT NULL,
    name character varying(256) NOT NULL,
    value text,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: category_custom_fields_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.category_custom_fields_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: category_custom_fields_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.category_custom_fields_id_seq OWNED BY public.category_custom_fields.id;


--
-- Name: category_featured_topics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.category_featured_topics (
    category_id integer NOT NULL,
    topic_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    rank integer DEFAULT 0 NOT NULL,
    id integer NOT NULL
);


--
-- Name: category_featured_topics_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.category_featured_topics_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: category_featured_topics_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.category_featured_topics_id_seq OWNED BY public.category_featured_topics.id;


--
-- Name: category_form_templates; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.category_form_templates (
    id bigint NOT NULL,
    category_id bigint NOT NULL,
    form_template_id bigint NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: category_form_templates_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.category_form_templates_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: category_form_templates_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.category_form_templates_id_seq OWNED BY public.category_form_templates.id;


--
-- Name: category_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.category_groups (
    id integer NOT NULL,
    category_id integer NOT NULL,
    group_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    permission_type integer DEFAULT 1
);


--
-- Name: category_groups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.category_groups_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: category_groups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.category_groups_id_seq OWNED BY public.category_groups.id;


--
-- Name: category_localizations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.category_localizations (
    id bigint NOT NULL,
    category_id bigint NOT NULL,
    locale character varying(20) NOT NULL,
    name character varying(50) NOT NULL,
    description character varying(1000),
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: category_localizations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.category_localizations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: category_localizations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.category_localizations_id_seq OWNED BY public.category_localizations.id;


--
-- Name: category_moderation_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.category_moderation_groups (
    id bigint NOT NULL,
    category_id integer,
    group_id integer,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: category_moderation_groups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.category_moderation_groups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: category_moderation_groups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.category_moderation_groups_id_seq OWNED BY public.category_moderation_groups.id;


--
-- Name: category_posting_review_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.category_posting_review_groups (
    id bigint NOT NULL,
    post_type integer NOT NULL,
    category_id integer NOT NULL,
    group_id integer NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: category_posting_review_groups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.category_posting_review_groups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: category_posting_review_groups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.category_posting_review_groups_id_seq OWNED BY public.category_posting_review_groups.id;


--
-- Name: category_required_tag_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.category_required_tag_groups (
    id bigint NOT NULL,
    category_id bigint NOT NULL,
    tag_group_id bigint NOT NULL,
    min_count integer DEFAULT 1 NOT NULL,
    "order" integer DEFAULT 1 NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: category_required_tag_groups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.category_required_tag_groups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: category_required_tag_groups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.category_required_tag_groups_id_seq OWNED BY public.category_required_tag_groups.id;


--
-- Name: category_search_data; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.category_search_data (
    category_id integer NOT NULL,
    search_data tsvector,
    raw_data text,
    locale text,
    version integer DEFAULT 0
);


--
-- Name: category_settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.category_settings (
    id bigint NOT NULL,
    category_id bigint NOT NULL,
    require_topic_approval boolean,
    require_reply_approval boolean,
    num_auto_bump_daily integer DEFAULT 0,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    auto_bump_cooldown_days integer DEFAULT 1,
    topic_posting_review_mode integer DEFAULT 0 NOT NULL,
    reply_posting_review_mode integer DEFAULT 0 NOT NULL,
    nested_replies_default boolean DEFAULT false NOT NULL
);


--
-- Name: category_settings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.category_settings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: category_settings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.category_settings_id_seq OWNED BY public.category_settings.id;


--
-- Name: category_tag_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.category_tag_groups (
    id integer NOT NULL,
    category_id integer NOT NULL,
    tag_group_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: category_tag_groups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.category_tag_groups_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: category_tag_groups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.category_tag_groups_id_seq OWNED BY public.category_tag_groups.id;


--
-- Name: category_tag_stats; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.category_tag_stats (
    id bigint NOT NULL,
    category_id bigint NOT NULL,
    tag_id bigint NOT NULL,
    topic_count integer DEFAULT 0 NOT NULL
);


--
-- Name: category_tag_stats_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.category_tag_stats_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: category_tag_stats_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.category_tag_stats_id_seq OWNED BY public.category_tag_stats.id;


--
-- Name: category_tags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.category_tags (
    id integer NOT NULL,
    category_id integer NOT NULL,
    tag_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: category_tags_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.category_tags_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: category_tags_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.category_tags_id_seq OWNED BY public.category_tags.id;


--
-- Name: category_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.category_users (
    id integer NOT NULL,
    category_id integer NOT NULL,
    user_id integer NOT NULL,
    notification_level integer NOT NULL,
    last_seen_at timestamp without time zone
);


--
-- Name: category_users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.category_users_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: category_users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.category_users_id_seq OWNED BY public.category_users.id;


--
-- Name: chat_channel_archives; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_channel_archives (
    id bigint NOT NULL,
    chat_channel_id bigint NOT NULL,
    archived_by_id integer NOT NULL,
    destination_topic_id integer,
    destination_topic_title character varying,
    destination_category_id integer,
    destination_tags character varying[],
    total_messages integer NOT NULL,
    archived_messages integer DEFAULT 0 NOT NULL,
    archive_error character varying,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: chat_channel_archives_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chat_channel_archives_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chat_channel_archives_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_channel_archives_id_seq OWNED BY public.chat_channel_archives.id;


--
-- Name: chat_channel_custom_fields; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_channel_custom_fields (
    id bigint NOT NULL,
    channel_id bigint NOT NULL,
    name character varying(256) NOT NULL,
    value character varying(1000000),
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: chat_channel_custom_fields_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chat_channel_custom_fields_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chat_channel_custom_fields_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_channel_custom_fields_id_seq OWNED BY public.chat_channel_custom_fields.id;


--
-- Name: chat_channels; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_channels (
    id bigint NOT NULL,
    chatable_id bigint NOT NULL,
    deleted_at timestamp without time zone,
    deleted_by_id integer,
    featured_in_category_id integer,
    delete_after_seconds integer,
    chatable_type character varying NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    name character varying,
    description text,
    status integer DEFAULT 0 NOT NULL,
    user_count integer DEFAULT 0 NOT NULL,
    auto_join_users boolean DEFAULT false NOT NULL,
    user_count_stale boolean DEFAULT false NOT NULL,
    type character varying,
    slug character varying,
    allow_channel_wide_mentions boolean DEFAULT true NOT NULL,
    messages_count integer DEFAULT 0 NOT NULL,
    threading_enabled boolean DEFAULT false NOT NULL,
    last_message_id bigint,
    emoji character varying
);


--
-- Name: chat_channels_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chat_channels_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chat_channels_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_channels_id_seq OWNED BY public.chat_channels.id;


--
-- Name: chat_drafts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_drafts (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    chat_channel_id bigint NOT NULL,
    data text NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    thread_id bigint
);


--
-- Name: chat_drafts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chat_drafts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chat_drafts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_drafts_id_seq OWNED BY public.chat_drafts.id;


--
-- Name: chat_mention_notifications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_mention_notifications (
    chat_mention_id bigint NOT NULL,
    notification_id bigint NOT NULL
);


--
-- Name: chat_mentions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_mentions (
    id bigint NOT NULL,
    chat_message_id bigint NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    type character varying NOT NULL,
    target_id integer
);


--
-- Name: chat_mentions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chat_mentions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chat_mentions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_mentions_id_seq OWNED BY public.chat_mentions.id;


--
-- Name: chat_message_custom_fields; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_message_custom_fields (
    id bigint NOT NULL,
    message_id bigint NOT NULL,
    name character varying(256) NOT NULL,
    value character varying(1000000),
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: chat_message_custom_fields_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chat_message_custom_fields_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chat_message_custom_fields_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_message_custom_fields_id_seq OWNED BY public.chat_message_custom_fields.id;


--
-- Name: chat_message_custom_prompts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_message_custom_prompts (
    id bigint NOT NULL,
    message_id bigint NOT NULL,
    custom_prompt json NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: chat_message_custom_prompts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chat_message_custom_prompts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chat_message_custom_prompts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_message_custom_prompts_id_seq OWNED BY public.chat_message_custom_prompts.id;


--
-- Name: chat_message_hotlinked_media; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_message_hotlinked_media (
    id bigint NOT NULL,
    chat_message_id bigint NOT NULL,
    url character varying NOT NULL,
    status character varying NOT NULL,
    upload_id bigint,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: chat_message_hotlinked_media_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chat_message_hotlinked_media_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chat_message_hotlinked_media_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_message_hotlinked_media_id_seq OWNED BY public.chat_message_hotlinked_media.id;


--
-- Name: chat_message_interactions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_message_interactions (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    chat_message_id bigint NOT NULL,
    action jsonb NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: chat_message_interactions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chat_message_interactions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chat_message_interactions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_message_interactions_id_seq OWNED BY public.chat_message_interactions.id;


--
-- Name: chat_message_links; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_message_links (
    id bigint NOT NULL,
    chat_message_id bigint NOT NULL,
    url character varying(500) NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: chat_message_links_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chat_message_links_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chat_message_links_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_message_links_id_seq OWNED BY public.chat_message_links.id;


--
-- Name: chat_message_reactions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_message_reactions (
    id bigint NOT NULL,
    chat_message_id bigint,
    user_id integer,
    emoji character varying,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: chat_message_reactions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chat_message_reactions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chat_message_reactions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_message_reactions_id_seq OWNED BY public.chat_message_reactions.id;


--
-- Name: chat_message_revisions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_message_revisions (
    id bigint NOT NULL,
    chat_message_id bigint,
    old_message text NOT NULL,
    new_message text NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    user_id integer NOT NULL
);


--
-- Name: chat_message_revisions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chat_message_revisions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chat_message_revisions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_message_revisions_id_seq OWNED BY public.chat_message_revisions.id;


--
-- Name: chat_message_search_data; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_message_search_data (
    chat_message_id bigint NOT NULL,
    search_data tsvector,
    raw_data text,
    locale text,
    version integer DEFAULT 0
);


--
-- Name: chat_message_search_data_chat_message_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chat_message_search_data_chat_message_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chat_message_search_data_chat_message_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_message_search_data_chat_message_id_seq OWNED BY public.chat_message_search_data.chat_message_id;


--
-- Name: chat_messages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_messages (
    id bigint NOT NULL,
    chat_channel_id bigint NOT NULL,
    user_id integer,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    deleted_at timestamp without time zone,
    deleted_by_id integer,
    in_reply_to_id bigint,
    message text,
    cooked text,
    cooked_version integer,
    last_editor_id integer NOT NULL,
    thread_id bigint,
    streaming boolean DEFAULT false NOT NULL,
    excerpt character varying(1000),
    created_by_sdk boolean DEFAULT false NOT NULL,
    blocks jsonb
);


--
-- Name: chat_messages_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chat_messages_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chat_messages_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_messages_id_seq OWNED BY public.chat_messages.id;


--
-- Name: chat_pinned_messages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_pinned_messages (
    id bigint NOT NULL,
    chat_message_id bigint NOT NULL,
    chat_channel_id bigint NOT NULL,
    pinned_by_id bigint NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: chat_pinned_messages_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chat_pinned_messages_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chat_pinned_messages_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_pinned_messages_id_seq OWNED BY public.chat_pinned_messages.id;


--
-- Name: chat_thread_custom_fields; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_thread_custom_fields (
    id bigint NOT NULL,
    thread_id bigint NOT NULL,
    name character varying(256) NOT NULL,
    value character varying(1000000),
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: chat_thread_custom_fields_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chat_thread_custom_fields_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chat_thread_custom_fields_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_thread_custom_fields_id_seq OWNED BY public.chat_thread_custom_fields.id;


--
-- Name: chat_threads; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_threads (
    id bigint NOT NULL,
    channel_id bigint NOT NULL,
    original_message_id bigint NOT NULL,
    original_message_user_id bigint NOT NULL,
    status integer DEFAULT 0 NOT NULL,
    title character varying,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    replies_count integer DEFAULT 0 NOT NULL,
    last_message_id bigint,
    force boolean DEFAULT false NOT NULL
);


--
-- Name: chat_threads_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chat_threads_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chat_threads_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_threads_id_seq OWNED BY public.chat_threads.id;


--
-- Name: chat_webhook_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_webhook_events (
    id bigint NOT NULL,
    chat_message_id bigint NOT NULL,
    incoming_chat_webhook_id bigint NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: chat_webhook_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.chat_webhook_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chat_webhook_events_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.chat_webhook_events_id_seq OWNED BY public.chat_webhook_events.id;


--
-- Name: child_themes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.child_themes (
    id integer NOT NULL,
    parent_theme_id integer,
    child_theme_id integer,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: child_themes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.child_themes_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: child_themes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.child_themes_id_seq OWNED BY public.child_themes.id;


--
-- Name: classification_results; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.classification_results (
    id bigint NOT NULL,
    model_used character varying,
    classification_type character varying,
    target_id bigint,
    target_type character varying,
    classification jsonb,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: classification_results_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.classification_results_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: classification_results_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.classification_results_id_seq OWNED BY public.classification_results.id;


--
-- Name: color_scheme_colors; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.color_scheme_colors (
    id integer NOT NULL,
    name character varying NOT NULL,
    hex character varying NOT NULL,
    color_scheme_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: color_scheme_colors_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.color_scheme_colors_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: color_scheme_colors_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.color_scheme_colors_id_seq OWNED BY public.color_scheme_colors.id;


--
-- Name: color_schemes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.color_schemes (
    id integer NOT NULL,
    name character varying NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    via_wizard boolean DEFAULT false NOT NULL,
    base_scheme_id integer,
    theme_id integer,
    user_selectable boolean DEFAULT false NOT NULL,
    remote_copy boolean DEFAULT false NOT NULL
);


--
-- Name: color_schemes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.color_schemes_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: color_schemes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.color_schemes_id_seq OWNED BY public.color_schemes.id;


--
-- Name: completion_prompts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.completion_prompts (
    id bigint NOT NULL,
    name character varying NOT NULL,
    translated_name character varying,
    prompt_type integer DEFAULT 0 NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    messages jsonb,
    temperature integer,
    stop_sequences character varying[]
);


--
-- Name: completion_prompts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.completion_prompts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: completion_prompts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.completion_prompts_id_seq OWNED BY public.completion_prompts.id;


--
-- Name: custom_emojis; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.custom_emojis (
    id integer NOT NULL,
    name character varying NOT NULL,
    upload_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    "group" character varying(20),
    user_id integer DEFAULT '-1'::integer NOT NULL
);


--
-- Name: custom_emojis_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.custom_emojis_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: custom_emojis_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.custom_emojis_id_seq OWNED BY public.custom_emojis.id;


--
-- Name: data_explorer_queries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.data_explorer_queries (
    id bigint NOT NULL,
    name character varying,
    description text,
    sql text DEFAULT 'SELECT 1'::text NOT NULL,
    user_id integer,
    last_run_at timestamp without time zone,
    hidden boolean DEFAULT false NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: data_explorer_queries_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.data_explorer_queries_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: data_explorer_queries_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.data_explorer_queries_id_seq OWNED BY public.data_explorer_queries.id;


--
-- Name: data_explorer_query_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.data_explorer_query_groups (
    id bigint NOT NULL,
    query_id bigint,
    group_id integer
);


--
-- Name: data_explorer_query_groups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.data_explorer_query_groups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: data_explorer_query_groups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.data_explorer_query_groups_id_seq OWNED BY public.data_explorer_query_groups.id;


--
-- Name: data_explorer_query_stats; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.data_explorer_query_stats (
    id bigint NOT NULL,
    query_id bigint NOT NULL,
    date date NOT NULL,
    total_runs integer DEFAULT 0 NOT NULL
);


--
-- Name: data_explorer_query_stats_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.data_explorer_query_stats_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: data_explorer_query_stats_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.data_explorer_query_stats_id_seq OWNED BY public.data_explorer_query_stats.id;


--
-- Name: developers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.developers (
    id integer NOT NULL,
    user_id integer NOT NULL
);


--
-- Name: developers_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.developers_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: developers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.developers_id_seq OWNED BY public.developers.id;


--
-- Name: direct_message_channels; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.direct_message_channels (
    id bigint NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    "group" boolean DEFAULT false NOT NULL
);


--
-- Name: direct_message_channels_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.direct_message_channels_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: direct_message_channels_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.direct_message_channels_id_seq OWNED BY public.direct_message_channels.id;


--
-- Name: direct_message_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.direct_message_users (
    id bigint NOT NULL,
    direct_message_channel_id bigint NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: direct_message_users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.direct_message_users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: direct_message_users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.direct_message_users_id_seq OWNED BY public.direct_message_users.id;


--
-- Name: directory_columns; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.directory_columns (
    id bigint NOT NULL,
    name character varying,
    automatic_position integer,
    icon character varying,
    user_field_id integer,
    enabled boolean NOT NULL,
    "position" integer NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    type integer DEFAULT 0 NOT NULL
);


--
-- Name: directory_columns_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.directory_columns_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: directory_columns_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.directory_columns_id_seq OWNED BY public.directory_columns.id;


--
-- Name: directory_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.directory_items (
    id integer NOT NULL,
    period_type integer NOT NULL,
    user_id integer NOT NULL,
    likes_received integer NOT NULL,
    likes_given integer NOT NULL,
    topics_entered integer NOT NULL,
    topic_count integer NOT NULL,
    post_count integer NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    days_visited integer DEFAULT 0 NOT NULL,
    posts_read integer DEFAULT 0 NOT NULL,
    solutions integer DEFAULT 0,
    gamification_score integer DEFAULT 0
);


--
-- Name: directory_items_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.directory_items_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: directory_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.directory_items_id_seq OWNED BY public.directory_items.id;


--
-- Name: discourse_ai_ai_bot_conversation_stars; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_ai_ai_bot_conversation_stars (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    topic_id integer NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_ai_ai_bot_conversation_stars_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_ai_ai_bot_conversation_stars_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_ai_ai_bot_conversation_stars_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_ai_ai_bot_conversation_stars_id_seq OWNED BY public.discourse_ai_ai_bot_conversation_stars.id;


--
-- Name: discourse_automation_automations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_automation_automations (
    id bigint NOT NULL,
    name character varying,
    script character varying NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    last_updated_by_id integer NOT NULL,
    trigger character varying
);


--
-- Name: discourse_automation_automations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_automation_automations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_automation_automations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_automation_automations_id_seq OWNED BY public.discourse_automation_automations.id;


--
-- Name: discourse_automation_fields; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_automation_fields (
    id bigint NOT NULL,
    automation_id bigint NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    component character varying NOT NULL,
    name character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    target character varying
);


--
-- Name: discourse_automation_fields_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_automation_fields_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_automation_fields_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_automation_fields_id_seq OWNED BY public.discourse_automation_fields.id;


--
-- Name: discourse_automation_pending_automations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_automation_pending_automations (
    id bigint NOT NULL,
    automation_id bigint NOT NULL,
    execute_at timestamp without time zone NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_automation_pending_automations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_automation_pending_automations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_automation_pending_automations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_automation_pending_automations_id_seq OWNED BY public.discourse_automation_pending_automations.id;


--
-- Name: discourse_automation_pending_pms; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_automation_pending_pms (
    id bigint NOT NULL,
    target_usernames character varying[],
    sender character varying,
    title character varying,
    raw character varying,
    automation_id bigint NOT NULL,
    execute_at timestamp without time zone NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    sender_id bigint,
    target_user_ids bigint[]
);


--
-- Name: discourse_automation_pending_pms_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_automation_pending_pms_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_automation_pending_pms_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_automation_pending_pms_id_seq OWNED BY public.discourse_automation_pending_pms.id;


--
-- Name: discourse_automation_stats; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_automation_stats (
    id bigint NOT NULL,
    automation_id bigint NOT NULL,
    date date NOT NULL,
    last_run_at timestamp(6) without time zone NOT NULL,
    total_time double precision NOT NULL,
    average_run_time double precision NOT NULL,
    min_run_time double precision NOT NULL,
    max_run_time double precision NOT NULL,
    total_runs integer NOT NULL,
    total_errors integer DEFAULT 0 NOT NULL
);


--
-- Name: discourse_automation_stats_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_automation_stats_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_automation_stats_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_automation_stats_id_seq OWNED BY public.discourse_automation_stats.id;


--
-- Name: discourse_automation_user_global_notices; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_automation_user_global_notices (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    notice text NOT NULL,
    identifier character varying NOT NULL,
    level character varying DEFAULT 'info'::character varying,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_automation_user_global_notices_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_automation_user_global_notices_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_automation_user_global_notices_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_automation_user_global_notices_id_seq OWNED BY public.discourse_automation_user_global_notices.id;


--
-- Name: discourse_calendar_disabled_holidays; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_calendar_disabled_holidays (
    id bigint NOT NULL,
    holiday_name character varying NOT NULL,
    region_code character varying NOT NULL,
    disabled boolean DEFAULT true NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_calendar_disabled_holidays_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_calendar_disabled_holidays_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_calendar_disabled_holidays_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_calendar_disabled_holidays_id_seq OWNED BY public.discourse_calendar_disabled_holidays.id;


--
-- Name: discourse_calendar_post_event_dates; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_calendar_post_event_dates (
    id bigint NOT NULL,
    event_id integer,
    starts_at timestamp without time zone,
    ends_at timestamp without time zone,
    reminder_counter integer DEFAULT 0,
    event_will_start_sent_at timestamp without time zone,
    event_started_sent_at timestamp without time zone,
    finished_at timestamp without time zone,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_calendar_post_event_dates_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_calendar_post_event_dates_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_calendar_post_event_dates_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_calendar_post_event_dates_id_seq OWNED BY public.discourse_calendar_post_event_dates.id;


--
-- Name: discourse_kanban_board_histories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_kanban_board_histories (
    id bigint NOT NULL,
    acting_user_id bigint NOT NULL,
    action integer NOT NULL,
    board_id bigint NOT NULL,
    column_id bigint,
    details jsonb,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_kanban_board_histories_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_kanban_board_histories_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_kanban_board_histories_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_kanban_board_histories_id_seq OWNED BY public.discourse_kanban_board_histories.id;


--
-- Name: discourse_kanban_boards; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_kanban_boards (
    id bigint NOT NULL,
    name character varying NOT NULL,
    slug character varying NOT NULL,
    allow_read_group_ids integer[] DEFAULT '{}'::integer[] NOT NULL,
    allow_write_group_ids integer[] DEFAULT '{}'::integer[] NOT NULL,
    require_confirmation boolean DEFAULT true NOT NULL,
    show_tags boolean DEFAULT false NOT NULL,
    card_style integer DEFAULT 0 NOT NULL,
    show_topic_thumbnail boolean DEFAULT false NOT NULL,
    created_by_id bigint,
    updated_by_id bigint,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    category_ids integer[] DEFAULT '{}'::integer[] NOT NULL,
    tag_ids integer[] DEFAULT '{}'::integer[] NOT NULL
);


--
-- Name: discourse_kanban_boards_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_kanban_boards_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_kanban_boards_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_kanban_boards_id_seq OWNED BY public.discourse_kanban_boards.id;


--
-- Name: discourse_kanban_card_histories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_kanban_card_histories (
    id bigint NOT NULL,
    acting_user_id bigint NOT NULL,
    action integer NOT NULL,
    board_id bigint NOT NULL,
    card_id bigint NOT NULL,
    details jsonb,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_kanban_card_histories_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_kanban_card_histories_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_kanban_card_histories_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_kanban_card_histories_id_seq OWNED BY public.discourse_kanban_card_histories.id;


--
-- Name: discourse_kanban_cards; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_kanban_cards (
    id bigint NOT NULL,
    board_id bigint NOT NULL,
    column_id bigint,
    topic_id bigint,
    card_type integer DEFAULT 0 NOT NULL,
    title character varying,
    notes text,
    due_at timestamp(6) without time zone,
    "position" bigint DEFAULT 0 NOT NULL,
    created_by_id bigint,
    updated_by_id bigint,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    assigned_to_id bigint,
    assigned_to_type character varying,
    tag_ids integer[] DEFAULT '{}'::integer[] NOT NULL,
    column_changed_at timestamp(6) without time zone NOT NULL,
    inline_onebox_data jsonb
);


--
-- Name: discourse_kanban_cards_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_kanban_cards_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_kanban_cards_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_kanban_cards_id_seq OWNED BY public.discourse_kanban_cards.id;


--
-- Name: discourse_kanban_columns; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_kanban_columns (
    id bigint NOT NULL,
    board_id bigint NOT NULL,
    title character varying NOT NULL,
    icon character varying,
    "position" integer DEFAULT 0 NOT NULL,
    move_to_category_id bigint,
    move_to_assigned character varying,
    move_to_status character varying,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    tag_id integer,
    default_sort integer DEFAULT 0 NOT NULL,
    color character varying
);


--
-- Name: discourse_kanban_columns_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_kanban_columns_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_kanban_columns_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_kanban_columns_id_seq OWNED BY public.discourse_kanban_columns.id;


--
-- Name: discourse_post_event_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_post_event_events (
    id bigint NOT NULL,
    status integer DEFAULT 0 NOT NULL,
    original_starts_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    original_ends_at timestamp without time zone,
    deleted_at timestamp without time zone,
    raw_invitees character varying[],
    name character varying,
    url character varying(1000),
    custom_fields jsonb DEFAULT '{}'::jsonb NOT NULL,
    reminders character varying,
    recurrence character varying,
    timezone character varying,
    minimal boolean,
    closed boolean DEFAULT false NOT NULL,
    chat_enabled boolean DEFAULT false NOT NULL,
    chat_channel_id bigint,
    recurrence_until timestamp(6) without time zone,
    show_local_time boolean DEFAULT false NOT NULL,
    location character varying(1000),
    description character varying(1000),
    max_attendees integer,
    all_day boolean DEFAULT false NOT NULL,
    image_upload_id bigint,
    livestream boolean DEFAULT false NOT NULL
);


--
-- Name: discourse_post_event_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_post_event_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_post_event_events_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_post_event_events_id_seq OWNED BY public.discourse_post_event_events.id;


--
-- Name: discourse_post_event_hosts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_post_event_hosts (
    id bigint NOT NULL,
    post_id bigint NOT NULL,
    user_id integer NOT NULL,
    "position" integer DEFAULT 0 NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_post_event_hosts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_post_event_hosts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_post_event_hosts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_post_event_hosts_id_seq OWNED BY public.discourse_post_event_hosts.id;


--
-- Name: discourse_post_event_invitees; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_post_event_invitees (
    id bigint NOT NULL,
    post_id integer NOT NULL,
    user_id integer NOT NULL,
    status integer,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    notified boolean DEFAULT false NOT NULL,
    recurring boolean DEFAULT false NOT NULL
);


--
-- Name: discourse_post_event_invitees_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_post_event_invitees_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_post_event_invitees_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_post_event_invitees_id_seq OWNED BY public.discourse_post_event_invitees.id;


--
-- Name: discourse_reactions_reaction_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_reactions_reaction_users (
    id bigint NOT NULL,
    reaction_id bigint,
    user_id integer,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    post_id integer
);


--
-- Name: discourse_reactions_reaction_users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_reactions_reaction_users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_reactions_reaction_users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_reactions_reaction_users_id_seq OWNED BY public.discourse_reactions_reaction_users.id;


--
-- Name: discourse_reactions_reactions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_reactions_reactions (
    id bigint NOT NULL,
    post_id integer,
    reaction_type integer,
    reaction_value character varying,
    reaction_users_count integer,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_reactions_reactions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_reactions_reactions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_reactions_reactions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_reactions_reactions_id_seq OWNED BY public.discourse_reactions_reactions.id;


--
-- Name: discourse_rss_polling_poll_attempts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_rss_polling_poll_attempts (
    id bigint NOT NULL,
    rss_feed_id bigint NOT NULL,
    status integer DEFAULT 0 NOT NULL,
    imported_count integer DEFAULT 0 NOT NULL,
    updated_count integer DEFAULT 0 NOT NULL,
    skipped_count integer DEFAULT 0 NOT NULL,
    failed_count integer DEFAULT 0 NOT NULL,
    error text,
    items jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_rss_polling_poll_attempts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_rss_polling_poll_attempts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_rss_polling_poll_attempts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_rss_polling_poll_attempts_id_seq OWNED BY public.discourse_rss_polling_poll_attempts.id;


--
-- Name: discourse_rss_polling_rss_feeds; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_rss_polling_rss_feeds (
    id bigint NOT NULL,
    url character varying NOT NULL,
    category_filter character varying,
    author character varying,
    category_id integer,
    tags character varying,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    user_id bigint,
    enabled boolean DEFAULT true NOT NULL
);


--
-- Name: discourse_rss_polling_rss_feeds_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_rss_polling_rss_feeds_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_rss_polling_rss_feeds_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_rss_polling_rss_feeds_id_seq OWNED BY public.discourse_rss_polling_rss_feeds.id;


--
-- Name: discourse_solved_shared_issues; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_solved_shared_issues (
    id bigint NOT NULL,
    topic_id integer NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_solved_shared_issues_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_solved_shared_issues_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_solved_shared_issues_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_solved_shared_issues_id_seq OWNED BY public.discourse_solved_shared_issues.id;


--
-- Name: discourse_solved_solved_topics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_solved_solved_topics (
    id bigint NOT NULL,
    topic_id bigint NOT NULL,
    answer_post_id integer,
    accepter_user_id integer,
    topic_timer_id integer,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_solved_solved_topics_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_solved_solved_topics_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_solved_solved_topics_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_solved_solved_topics_id_seq OWNED BY public.discourse_solved_solved_topics.id;


--
-- Name: discourse_solved_topic_answers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_solved_topic_answers (
    id bigint NOT NULL,
    solved_topic_id bigint NOT NULL,
    answer_post_id bigint NOT NULL,
    accepter_user_id bigint NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_solved_topic_answers_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_solved_topic_answers_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_solved_topic_answers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_solved_topic_answers_id_seq OWNED BY public.discourse_solved_topic_answers.id;


--
-- Name: discourse_subscriptions_customers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_subscriptions_customers (
    id bigint NOT NULL,
    customer_id character varying NOT NULL,
    product_id character varying,
    user_id bigint,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_subscriptions_customers_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_subscriptions_customers_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_subscriptions_customers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_subscriptions_customers_id_seq OWNED BY public.discourse_subscriptions_customers.id;


--
-- Name: discourse_subscriptions_products; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_subscriptions_products (
    id bigint NOT NULL,
    external_id character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_subscriptions_products_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_subscriptions_products_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_subscriptions_products_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_subscriptions_products_id_seq OWNED BY public.discourse_subscriptions_products.id;


--
-- Name: discourse_subscriptions_subscriptions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_subscriptions_subscriptions (
    id bigint NOT NULL,
    customer_id bigint NOT NULL,
    external_id character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    status character varying
);


--
-- Name: discourse_subscriptions_subscriptions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_subscriptions_subscriptions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_subscriptions_subscriptions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_subscriptions_subscriptions_id_seq OWNED BY public.discourse_subscriptions_subscriptions.id;


--
-- Name: discourse_templates_usage_count; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_templates_usage_count (
    id bigint NOT NULL,
    topic_id integer NOT NULL,
    usage_count integer DEFAULT 0 NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_templates_usage_count_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_templates_usage_count_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_templates_usage_count_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_templates_usage_count_id_seq OWNED BY public.discourse_templates_usage_count.id;


--
-- Name: discourse_workflows_ai_authoring_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_workflows_ai_authoring_sessions (
    id bigint NOT NULL,
    workflow_id bigint,
    user_id integer NOT NULL,
    status character varying(40) DEFAULT 'drafting'::character varying NOT NULL,
    messages jsonb DEFAULT '[]'::jsonb NOT NULL,
    latest_request text,
    latest_response jsonb DEFAULT '{}'::jsonb NOT NULL,
    proposed_patch jsonb DEFAULT '{}'::jsonb NOT NULL,
    base_workflow_version_id character varying(36),
    base_graph_digest character varying(64),
    risk_level character varying(20),
    applied_at timestamp(6) without time zone,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_workflows_ai_authoring_sessions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_workflows_ai_authoring_sessions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_workflows_ai_authoring_sessions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_workflows_ai_authoring_sessions_id_seq OWNED BY public.discourse_workflows_ai_authoring_sessions.id;


--
-- Name: discourse_workflows_credentials; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_workflows_credentials (
    id bigint NOT NULL,
    name character varying(128) NOT NULL,
    credential_type character varying(64) NOT NULL,
    data jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_by_id integer,
    updated_by_id integer,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_workflows_credentials_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_workflows_credentials_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_workflows_credentials_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_workflows_credentials_id_seq OWNED BY public.discourse_workflows_credentials.id;


--
-- Name: discourse_workflows_data_tables; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_workflows_data_tables (
    id bigint NOT NULL,
    name character varying(100) NOT NULL,
    created_by_id integer,
    updated_by_id integer,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_workflows_data_tables_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_workflows_data_tables_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_workflows_data_tables_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_workflows_data_tables_id_seq OWNED BY public.discourse_workflows_data_tables.id;


--
-- Name: discourse_workflows_execution_data; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_workflows_execution_data (
    execution_id bigint NOT NULL,
    data jsonb DEFAULT '{}'::jsonb NOT NULL,
    workflow_data jsonb DEFAULT '{}'::jsonb NOT NULL
);


--
-- Name: discourse_workflows_execution_stats; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_workflows_execution_stats (
    id bigint NOT NULL,
    workflow_id bigint NOT NULL,
    date date NOT NULL,
    total_runs integer DEFAULT 0 NOT NULL
);


--
-- Name: discourse_workflows_execution_stats_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_workflows_execution_stats_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_workflows_execution_stats_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_workflows_execution_stats_id_seq OWNED BY public.discourse_workflows_execution_stats.id;


--
-- Name: discourse_workflows_executions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_workflows_executions (
    id bigint NOT NULL,
    workflow_id bigint NOT NULL,
    workflow_version_id character varying(36) NOT NULL,
    status integer DEFAULT 0 NOT NULL,
    execution_mode integer DEFAULT 0 NOT NULL,
    trigger_data jsonb DEFAULT '{}'::jsonb,
    error text,
    waiting_node_id character varying(100),
    waiting_until timestamp(6) without time zone,
    resume_token character varying(64),
    timeout_action character varying(32),
    trigger_node_id character varying(100),
    run_time_ms integer,
    started_at timestamp(6) without time zone,
    finished_at timestamp(6) without time zone,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_workflows_executions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_workflows_executions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_workflows_executions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_workflows_executions_id_seq OWNED BY public.discourse_workflows_executions.id;


--
-- Name: discourse_workflows_tags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_workflows_tags (
    id bigint NOT NULL,
    name character varying(100) NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_workflows_tags_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_workflows_tags_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_workflows_tags_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_workflows_tags_id_seq OWNED BY public.discourse_workflows_tags.id;


--
-- Name: discourse_workflows_variables; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_workflows_variables (
    id bigint NOT NULL,
    key character varying(100) NOT NULL,
    value character varying(1000) DEFAULT ''::character varying NOT NULL,
    description text,
    created_by_id integer NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_workflows_variables_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_workflows_variables_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_workflows_variables_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_workflows_variables_id_seq OWNED BY public.discourse_workflows_variables.id;


--
-- Name: discourse_workflows_webhooks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_workflows_webhooks (
    id bigint NOT NULL,
    workflow_id bigint NOT NULL,
    workflow_version_id character varying(36),
    node_name character varying(100) NOT NULL,
    webhook_path character varying(500) NOT NULL,
    http_method character varying(10) NOT NULL,
    webhook_id character varying(36),
    path_length integer,
    test_webhook boolean DEFAULT false NOT NULL,
    user_id integer,
    workflow_snapshot jsonb,
    expires_at timestamp(6) without time zone,
    created_at timestamp(6) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: discourse_workflows_webhooks_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_workflows_webhooks_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_workflows_webhooks_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_workflows_webhooks_id_seq OWNED BY public.discourse_workflows_webhooks.id;


--
-- Name: discourse_workflows_workflow_call_runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_workflows_workflow_call_runs (
    id bigint NOT NULL,
    parent_execution_id bigint NOT NULL,
    parent_node_id character varying(100) NOT NULL,
    parent_resume_token character varying(64) NOT NULL,
    child_execution_id bigint,
    target_workflow_id bigint NOT NULL,
    target_workflow_version_id character varying(36) NOT NULL,
    user_id bigint,
    trigger_data jsonb DEFAULT '{}'::jsonb NOT NULL,
    status integer DEFAULT 0 NOT NULL,
    error text,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_workflows_workflow_call_runs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_workflows_workflow_call_runs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_workflows_workflow_call_runs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_workflows_workflow_call_runs_id_seq OWNED BY public.discourse_workflows_workflow_call_runs.id;


--
-- Name: discourse_workflows_workflow_dependencies; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_workflows_workflow_dependencies (
    id bigint NOT NULL,
    workflow_id bigint NOT NULL,
    dependency_type character varying(50) NOT NULL,
    dependency_key character varying(500) NOT NULL,
    node_id character varying(100),
    workflow_version_id character varying(36),
    created_at timestamp(6) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: discourse_workflows_workflow_dependencies_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_workflows_workflow_dependencies_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_workflows_workflow_dependencies_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_workflows_workflow_dependencies_id_seq OWNED BY public.discourse_workflows_workflow_dependencies.id;


--
-- Name: discourse_workflows_workflow_publish_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_workflows_workflow_publish_history (
    id bigint NOT NULL,
    workflow_id bigint NOT NULL,
    version_id character varying(36),
    event character varying(32) NOT NULL,
    user_id integer,
    created_at timestamp(6) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: discourse_workflows_workflow_publish_history_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_workflows_workflow_publish_history_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_workflows_workflow_publish_history_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_workflows_workflow_publish_history_id_seq OWNED BY public.discourse_workflows_workflow_publish_history.id;


--
-- Name: discourse_workflows_workflow_tags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_workflows_workflow_tags (
    id bigint NOT NULL,
    workflow_id bigint NOT NULL,
    workflow_tag_id bigint NOT NULL,
    created_at timestamp(6) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: discourse_workflows_workflow_tags_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_workflows_workflow_tags_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_workflows_workflow_tags_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_workflows_workflow_tags_id_seq OWNED BY public.discourse_workflows_workflow_tags.id;


--
-- Name: discourse_workflows_workflow_versions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_workflows_workflow_versions (
    version_id character varying(36) NOT NULL,
    workflow_id bigint NOT NULL,
    version_number integer NOT NULL,
    name character varying(100) NOT NULL,
    nodes jsonb DEFAULT '[]'::jsonb NOT NULL,
    connections jsonb DEFAULT '{}'::jsonb NOT NULL,
    settings jsonb DEFAULT '{}'::jsonb NOT NULL,
    autosaved boolean DEFAULT false NOT NULL,
    authors text,
    created_by_id integer NOT NULL,
    updated_by_id integer,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_workflows_workflows; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.discourse_workflows_workflows (
    id bigint NOT NULL,
    name character varying(100) NOT NULL,
    nodes jsonb DEFAULT '[]'::jsonb NOT NULL,
    connections jsonb DEFAULT '{}'::jsonb NOT NULL,
    static_data jsonb DEFAULT '{}'::jsonb NOT NULL,
    pin_data jsonb DEFAULT '{}'::jsonb NOT NULL,
    trigger_state jsonb DEFAULT '{}'::jsonb NOT NULL,
    settings jsonb DEFAULT '{}'::jsonb NOT NULL,
    version_id character varying(36) NOT NULL,
    active_version_id character varying(36),
    version_counter integer DEFAULT 1 NOT NULL,
    error_workflow_id bigint,
    created_by_id integer NOT NULL,
    updated_by_id integer,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: discourse_workflows_workflows_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.discourse_workflows_workflows_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: discourse_workflows_workflows_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.discourse_workflows_workflows_id_seq OWNED BY public.discourse_workflows_workflows.id;


--
-- Name: dismissed_topic_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dismissed_topic_users (
    id bigint NOT NULL,
    user_id integer,
    topic_id integer,
    created_at timestamp without time zone
);


--
-- Name: dismissed_topic_users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.dismissed_topic_users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: dismissed_topic_users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.dismissed_topic_users_id_seq OWNED BY public.dismissed_topic_users.id;


--
-- Name: do_not_disturb_timings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.do_not_disturb_timings (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    starts_at timestamp without time zone NOT NULL,
    ends_at timestamp without time zone NOT NULL,
    scheduled boolean DEFAULT false
);


--
-- Name: do_not_disturb_timings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.do_not_disturb_timings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: do_not_disturb_timings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.do_not_disturb_timings_id_seq OWNED BY public.do_not_disturb_timings.id;


--
-- Name: draft_sequences; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.draft_sequences (
    id integer NOT NULL,
    user_id integer NOT NULL,
    draft_key character varying NOT NULL,
    sequence bigint NOT NULL
);


--
-- Name: draft_sequences_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.draft_sequences_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: draft_sequences_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.draft_sequences_id_seq OWNED BY public.draft_sequences.id;


--
-- Name: drafts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drafts (
    id integer NOT NULL,
    user_id integer NOT NULL,
    draft_key character varying NOT NULL,
    data text NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    sequence bigint DEFAULT 0 NOT NULL,
    revisions integer DEFAULT 1 NOT NULL,
    owner character varying
);


--
-- Name: drafts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drafts_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drafts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drafts_id_seq OWNED BY public.drafts.id;


--
-- Name: email_change_requests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.email_change_requests (
    id integer NOT NULL,
    user_id integer NOT NULL,
    old_email character varying,
    new_email character varying NOT NULL,
    old_email_token_id integer,
    new_email_token_id integer,
    change_state integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    requested_by_user_id integer
);


--
-- Name: email_change_requests_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.email_change_requests_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: email_change_requests_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.email_change_requests_id_seq OWNED BY public.email_change_requests.id;


--
-- Name: email_login_codes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.email_login_codes (
    id bigint NOT NULL,
    email character varying NOT NULL,
    code_hash character varying NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    expires_at timestamp(6) without time zone NOT NULL,
    consumed_at timestamp(6) without time zone,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: email_login_codes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.email_login_codes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: email_login_codes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.email_login_codes_id_seq OWNED BY public.email_login_codes.id;


--
-- Name: email_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.email_logs (
    id integer NOT NULL,
    to_address character varying NOT NULL,
    email_type character varying NOT NULL,
    user_id integer,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    post_id integer,
    bounce_key uuid,
    bounced boolean DEFAULT false NOT NULL,
    message_id character varying,
    smtp_group_id integer,
    cc_addresses text,
    cc_user_ids integer[],
    raw text,
    topic_id integer,
    bounce_error_code character varying,
    smtp_transaction_response character varying(500),
    bcc_addresses text
);


--
-- Name: email_logs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.email_logs_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: email_logs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.email_logs_id_seq OWNED BY public.email_logs.id;


--
-- Name: email_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.email_tokens (
    id integer NOT NULL,
    user_id integer NOT NULL,
    email character varying NOT NULL,
    confirmed boolean DEFAULT false NOT NULL,
    expired boolean DEFAULT false NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    token_hash character varying NOT NULL,
    scope integer
);


--
-- Name: email_tokens_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.email_tokens_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: email_tokens_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.email_tokens_id_seq OWNED BY public.email_tokens.id;


--
-- Name: embeddable_host_tags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.embeddable_host_tags (
    id bigint NOT NULL,
    embeddable_host_id integer NOT NULL,
    tag_id integer NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: embeddable_host_tags_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.embeddable_host_tags_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: embeddable_host_tags_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.embeddable_host_tags_id_seq OWNED BY public.embeddable_host_tags.id;


--
-- Name: embeddable_hosts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.embeddable_hosts (
    id integer NOT NULL,
    host character varying NOT NULL,
    category_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    class_name character varying,
    allowed_paths character varying,
    user_id integer
);


--
-- Name: embeddable_hosts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.embeddable_hosts_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: embeddable_hosts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.embeddable_hosts_id_seq OWNED BY public.embeddable_hosts.id;


--
-- Name: embedding_definitions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.embedding_definitions (
    id bigint NOT NULL,
    display_name character varying NOT NULL,
    dimensions integer NOT NULL,
    max_sequence_length integer NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    pg_function character varying NOT NULL,
    provider character varying NOT NULL,
    tokenizer_class character varying NOT NULL,
    url character varying NOT NULL,
    api_key character varying,
    seeded boolean DEFAULT false NOT NULL,
    provider_params jsonb,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    embed_prompt character varying DEFAULT ''::character varying NOT NULL,
    search_prompt character varying DEFAULT ''::character varying NOT NULL,
    matryoshka_dimensions boolean DEFAULT false NOT NULL,
    ai_secret_id bigint
);


--
-- Name: embedding_definitions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.embedding_definitions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: embedding_definitions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.embedding_definitions_id_seq OWNED BY public.embedding_definitions.id;


--
-- Name: external_upload_stubs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.external_upload_stubs (
    id bigint NOT NULL,
    key character varying NOT NULL,
    original_filename character varying NOT NULL,
    status integer DEFAULT 1 NOT NULL,
    unique_identifier uuid NOT NULL,
    created_by_id integer NOT NULL,
    upload_type character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    multipart boolean DEFAULT false NOT NULL,
    external_upload_identifier character varying,
    filesize bigint NOT NULL
);


--
-- Name: external_upload_stubs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.external_upload_stubs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: external_upload_stubs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.external_upload_stubs_id_seq OWNED BY public.external_upload_stubs.id;


--
-- Name: flags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.flags (
    id bigint NOT NULL,
    name character varying,
    name_key character varying,
    description text,
    notify_type boolean DEFAULT false NOT NULL,
    auto_action_type boolean DEFAULT false NOT NULL,
    applies_to character varying[] NOT NULL,
    "position" integer NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    score_type boolean DEFAULT false NOT NULL,
    require_message boolean DEFAULT false NOT NULL
);


--
-- Name: flags_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.flags_id_seq
    START WITH 1001
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: flags_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.flags_id_seq OWNED BY public.flags.id;


--
-- Name: form_templates; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.form_templates (
    id bigint NOT NULL,
    name character varying NOT NULL,
    template text NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: form_templates_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.form_templates_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: form_templates_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.form_templates_id_seq OWNED BY public.form_templates.id;


--
-- Name: gamification_leaderboard_scores; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.gamification_leaderboard_scores (
    id bigint NOT NULL,
    leaderboard_id bigint NOT NULL,
    user_id bigint NOT NULL,
    date date NOT NULL,
    score integer DEFAULT 0 NOT NULL
);


--
-- Name: gamification_leaderboard_scores_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.gamification_leaderboard_scores_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gamification_leaderboard_scores_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.gamification_leaderboard_scores_id_seq OWNED BY public.gamification_leaderboard_scores.id;


--
-- Name: gamification_leaderboards; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.gamification_leaderboards (
    id bigint NOT NULL,
    name character varying NOT NULL,
    from_date date,
    to_date date,
    for_category_id integer,
    created_by_id integer NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    visible_to_groups_ids integer[] DEFAULT '{}'::integer[] NOT NULL,
    included_groups_ids integer[] DEFAULT '{}'::integer[] NOT NULL,
    excluded_groups_ids integer[] DEFAULT '{}'::integer[] NOT NULL,
    default_period integer DEFAULT 0,
    period_filter_disabled boolean DEFAULT false NOT NULL,
    score_overrides jsonb,
    scorable_category_ids integer[]
);


--
-- Name: gamification_leaderboards_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.gamification_leaderboards_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gamification_leaderboards_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.gamification_leaderboards_id_seq OWNED BY public.gamification_leaderboards.id;


--
-- Name: gamification_score_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.gamification_score_events (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    date date NOT NULL,
    points integer NOT NULL,
    description text,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: gamification_score_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.gamification_score_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gamification_score_events_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.gamification_score_events_id_seq OWNED BY public.gamification_score_events.id;


--
-- Name: gamification_scores; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.gamification_scores (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    date date NOT NULL,
    score integer NOT NULL
);


--
-- Name: gamification_scores_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.gamification_scores_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: gamification_scores_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.gamification_scores_id_seq OWNED BY public.gamification_scores.id;


--
-- Name: github_commits; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.github_commits (
    id bigint NOT NULL,
    repo_id bigint NOT NULL,
    sha character varying(40) NOT NULL,
    email character varying(513) NOT NULL,
    committed_at timestamp without time zone NOT NULL,
    role_id integer NOT NULL,
    merge_commit boolean DEFAULT false NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: github_commits_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.github_commits_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: github_commits_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.github_commits_id_seq OWNED BY public.github_commits.id;


--
-- Name: github_repos; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.github_repos (
    id bigint NOT NULL,
    name character varying(255) NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: github_repos_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.github_repos_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: github_repos_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.github_repos_id_seq OWNED BY public.github_repos.id;


--
-- Name: given_daily_likes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.given_daily_likes (
    user_id integer NOT NULL,
    likes_given integer NOT NULL,
    given_date date NOT NULL,
    limit_reached boolean DEFAULT false NOT NULL
);


--
-- Name: group_archived_messages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.group_archived_messages (
    id integer NOT NULL,
    group_id integer NOT NULL,
    topic_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: group_archived_messages_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.group_archived_messages_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: group_archived_messages_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.group_archived_messages_id_seq OWNED BY public.group_archived_messages.id;


--
-- Name: group_associated_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.group_associated_groups (
    id bigint NOT NULL,
    group_id bigint NOT NULL,
    associated_group_id bigint NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: group_associated_groups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.group_associated_groups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: group_associated_groups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.group_associated_groups_id_seq OWNED BY public.group_associated_groups.id;


--
-- Name: group_category_notification_defaults; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.group_category_notification_defaults (
    id bigint NOT NULL,
    group_id integer NOT NULL,
    category_id integer NOT NULL,
    notification_level integer NOT NULL
);


--
-- Name: group_category_notification_defaults_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.group_category_notification_defaults_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: group_category_notification_defaults_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.group_category_notification_defaults_id_seq OWNED BY public.group_category_notification_defaults.id;


--
-- Name: group_custom_fields; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.group_custom_fields (
    id integer NOT NULL,
    group_id integer NOT NULL,
    name character varying(256) NOT NULL,
    value text,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: group_custom_fields_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.group_custom_fields_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: group_custom_fields_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.group_custom_fields_id_seq OWNED BY public.group_custom_fields.id;


--
-- Name: group_histories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.group_histories (
    id integer NOT NULL,
    group_id integer NOT NULL,
    acting_user_id integer NOT NULL,
    target_user_id integer,
    action integer NOT NULL,
    subject character varying,
    prev_value text,
    new_value text,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: group_histories_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.group_histories_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: group_histories_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.group_histories_id_seq OWNED BY public.group_histories.id;


--
-- Name: group_mentions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.group_mentions (
    id integer NOT NULL,
    post_id integer,
    group_id integer,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: group_mentions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.group_mentions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: group_mentions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.group_mentions_id_seq OWNED BY public.group_mentions.id;


--
-- Name: group_requests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.group_requests (
    id bigint NOT NULL,
    group_id integer,
    user_id integer,
    reason text,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: group_requests_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.group_requests_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: group_requests_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.group_requests_id_seq OWNED BY public.group_requests.id;


--
-- Name: group_tag_notification_defaults; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.group_tag_notification_defaults (
    id bigint NOT NULL,
    group_id integer NOT NULL,
    tag_id integer NOT NULL,
    notification_level integer NOT NULL
);


--
-- Name: group_tag_notification_defaults_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.group_tag_notification_defaults_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: group_tag_notification_defaults_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.group_tag_notification_defaults_id_seq OWNED BY public.group_tag_notification_defaults.id;


--
-- Name: group_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.group_users (
    id integer NOT NULL,
    group_id integer NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    owner boolean DEFAULT false NOT NULL,
    notification_level integer DEFAULT 2 NOT NULL,
    first_unread_pm_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: group_users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.group_users_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: group_users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.group_users_id_seq OWNED BY public.group_users.id;


--
-- Name: groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.groups (
    id integer NOT NULL,
    name character varying NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    automatic boolean DEFAULT false NOT NULL,
    user_count integer DEFAULT 0 NOT NULL,
    automatic_membership_email_domains text,
    primary_group boolean DEFAULT false NOT NULL,
    title character varying,
    grant_trust_level integer,
    incoming_email character varying,
    has_messages boolean DEFAULT false NOT NULL,
    flair_bg_color character varying,
    flair_color character varying,
    bio_raw text,
    bio_cooked text,
    allow_membership_requests boolean DEFAULT false NOT NULL,
    full_name character varying,
    default_notification_level integer DEFAULT 3 NOT NULL,
    visibility_level integer DEFAULT 0 NOT NULL,
    public_exit boolean DEFAULT false NOT NULL,
    public_admission boolean DEFAULT false NOT NULL,
    membership_request_template text,
    messageable_level integer DEFAULT 0,
    mentionable_level integer DEFAULT 0,
    smtp_server character varying,
    smtp_port integer,
    imap_server character varying,
    imap_port integer,
    imap_ssl boolean,
    imap_mailbox_name character varying DEFAULT ''::character varying NOT NULL,
    imap_uid_validity integer DEFAULT 0 NOT NULL,
    imap_last_uid integer DEFAULT 0 NOT NULL,
    email_username character varying,
    email_password character varying,
    publish_read_state boolean DEFAULT false NOT NULL,
    members_visibility_level integer DEFAULT 0 NOT NULL,
    imap_last_error text,
    imap_old_emails integer,
    imap_new_emails integer,
    flair_icon character varying,
    flair_upload_id integer,
    allow_unknown_sender_topic_replies boolean DEFAULT false NOT NULL,
    smtp_enabled boolean DEFAULT false,
    smtp_updated_at timestamp without time zone,
    smtp_updated_by_id integer,
    imap_enabled boolean DEFAULT false,
    imap_updated_at timestamp without time zone,
    imap_updated_by_id integer,
    assignable_level integer DEFAULT 0 NOT NULL,
    email_from_alias character varying,
    smtp_ssl_mode integer DEFAULT 0 NOT NULL
);


--
-- Name: groups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.groups_id_seq
    AS integer
    START WITH 40
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: groups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.groups_id_seq OWNED BY public.groups.id;


--
-- Name: groups_web_hooks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.groups_web_hooks (
    web_hook_id integer NOT NULL,
    group_id integer NOT NULL
);


--
-- Name: ignored_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ignored_users (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    ignored_user_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    summarized_at timestamp without time zone,
    expiring_at timestamp without time zone NOT NULL
);


--
-- Name: ignored_users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ignored_users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ignored_users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ignored_users_id_seq OWNED BY public.ignored_users.id;


--
-- Name: incoming_chat_webhooks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.incoming_chat_webhooks (
    id bigint NOT NULL,
    name character varying NOT NULL,
    key character varying NOT NULL,
    chat_channel_id bigint NOT NULL,
    username character varying,
    description character varying,
    emoji character varying,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: incoming_chat_webhooks_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.incoming_chat_webhooks_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: incoming_chat_webhooks_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.incoming_chat_webhooks_id_seq OWNED BY public.incoming_chat_webhooks.id;


--
-- Name: incoming_domains; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.incoming_domains (
    id integer NOT NULL,
    name character varying(100) NOT NULL,
    https boolean DEFAULT false NOT NULL,
    port integer NOT NULL
);


--
-- Name: incoming_domains_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.incoming_domains_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: incoming_domains_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.incoming_domains_id_seq OWNED BY public.incoming_domains.id;


--
-- Name: incoming_emails; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.incoming_emails (
    id integer NOT NULL,
    user_id integer,
    topic_id integer,
    post_id integer,
    raw text,
    error text,
    message_id text,
    from_address text,
    to_addresses text,
    cc_addresses text,
    subject text,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    rejection_message text,
    is_auto_generated boolean DEFAULT false,
    is_bounce boolean DEFAULT false NOT NULL,
    imap_uid_validity integer,
    imap_uid integer,
    imap_sync boolean,
    imap_group_id bigint,
    imap_missing boolean DEFAULT false NOT NULL,
    created_via integer DEFAULT 0 NOT NULL
);


--
-- Name: incoming_emails_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.incoming_emails_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: incoming_emails_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.incoming_emails_id_seq OWNED BY public.incoming_emails.id;


--
-- Name: incoming_links; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.incoming_links (
    id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    user_id integer,
    ip_address inet,
    current_user_id integer,
    post_id integer NOT NULL,
    incoming_referer_id integer
);


--
-- Name: incoming_links_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.incoming_links_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: incoming_links_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.incoming_links_id_seq OWNED BY public.incoming_links.id;


--
-- Name: incoming_referers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.incoming_referers (
    id integer NOT NULL,
    path character varying(1000) NOT NULL,
    incoming_domain_id integer NOT NULL
);


--
-- Name: incoming_referers_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.incoming_referers_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: incoming_referers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.incoming_referers_id_seq OWNED BY public.incoming_referers.id;


--
-- Name: inferred_concept_posts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.inferred_concept_posts (
    inferred_concept_id bigint,
    post_id bigint,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: inferred_concept_topics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.inferred_concept_topics (
    inferred_concept_id bigint,
    topic_id bigint,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: inferred_concepts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.inferred_concepts (
    id bigint NOT NULL,
    name character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: inferred_concepts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.inferred_concepts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: inferred_concepts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.inferred_concepts_id_seq OWNED BY public.inferred_concepts.id;


--
-- Name: invited_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.invited_groups (
    id integer NOT NULL,
    group_id integer,
    invite_id integer,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: invited_groups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.invited_groups_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: invited_groups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.invited_groups_id_seq OWNED BY public.invited_groups.id;


--
-- Name: invited_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.invited_users (
    id bigint NOT NULL,
    user_id integer,
    invite_id integer NOT NULL,
    redeemed_at timestamp without time zone,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: invited_users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.invited_users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: invited_users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.invited_users_id_seq OWNED BY public.invited_users.id;


--
-- Name: invites; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.invites (
    id integer NOT NULL,
    invite_key character varying(32) NOT NULL,
    email character varying,
    invited_by_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    deleted_at timestamp without time zone,
    deleted_by_id integer,
    invalidated_at timestamp without time zone,
    moderator boolean DEFAULT false NOT NULL,
    custom_message text,
    emailed_status integer,
    max_redemptions_allowed integer DEFAULT 1 NOT NULL,
    redemption_count integer DEFAULT 0 NOT NULL,
    expires_at timestamp without time zone NOT NULL,
    email_token character varying,
    domain character varying,
    description character varying(100),
    admin boolean DEFAULT false NOT NULL
);


--
-- Name: invites_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.invites_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: invites_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.invites_id_seq OWNED BY public.invites.id;


--
-- Name: javascript_caches; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.javascript_caches (
    id bigint NOT NULL,
    theme_field_id bigint,
    digest character varying,
    content text NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    theme_id bigint,
    source_map text,
    name character varying,
    external_plugin_imports character varying[] DEFAULT '{}'::character varying[] NOT NULL,
    CONSTRAINT enforce_theme_or_theme_field CHECK ((((theme_id IS NOT NULL) AND (theme_field_id IS NULL)) OR ((theme_id IS NULL) AND (theme_field_id IS NOT NULL))))
);


--
-- Name: javascript_caches_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.javascript_caches_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: javascript_caches_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.javascript_caches_id_seq OWNED BY public.javascript_caches.id;


--
-- Name: linked_topics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.linked_topics (
    id bigint NOT NULL,
    topic_id bigint NOT NULL,
    original_topic_id bigint NOT NULL,
    sequence integer NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: linked_topics_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.linked_topics_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: linked_topics_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.linked_topics_id_seq OWNED BY public.linked_topics.id;


--
-- Name: livestream_topic_chat_channels; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.livestream_topic_chat_channels (
    id bigint NOT NULL,
    topic_id bigint NOT NULL,
    chat_channel_id bigint NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    reference_message_id bigint
);


--
-- Name: livestream_topic_chat_channels_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.livestream_topic_chat_channels_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: livestream_topic_chat_channels_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.livestream_topic_chat_channels_id_seq OWNED BY public.livestream_topic_chat_channels.id;


--
-- Name: llm_credit_allocations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.llm_credit_allocations (
    id bigint NOT NULL,
    llm_model_id bigint NOT NULL,
    soft_limit_percentage integer DEFAULT 80 NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    daily_credits bigint DEFAULT 0 NOT NULL
);


--
-- Name: llm_credit_allocations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.llm_credit_allocations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: llm_credit_allocations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.llm_credit_allocations_id_seq OWNED BY public.llm_credit_allocations.id;


--
-- Name: llm_credit_daily_usages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.llm_credit_daily_usages (
    id bigint NOT NULL,
    llm_model_id bigint NOT NULL,
    usage_date date NOT NULL,
    credits_used bigint DEFAULT 0 NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: llm_credit_daily_usages_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.llm_credit_daily_usages_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: llm_credit_daily_usages_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.llm_credit_daily_usages_id_seq OWNED BY public.llm_credit_daily_usages.id;


--
-- Name: llm_feature_credit_costs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.llm_feature_credit_costs (
    id bigint NOT NULL,
    llm_model_id bigint NOT NULL,
    feature_name character varying NOT NULL,
    credits_per_token numeric(10,4) DEFAULT 1.0 NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: llm_feature_credit_costs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.llm_feature_credit_costs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: llm_feature_credit_costs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.llm_feature_credit_costs_id_seq OWNED BY public.llm_feature_credit_costs.id;


--
-- Name: llm_models; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.llm_models (
    id bigint NOT NULL,
    display_name character varying,
    name character varying NOT NULL,
    provider character varying NOT NULL,
    tokenizer character varying NOT NULL,
    max_prompt_tokens integer NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    url character varying,
    api_key character varying,
    user_id integer,
    enabled_chat_bot boolean DEFAULT false NOT NULL,
    provider_params jsonb DEFAULT '{}'::jsonb,
    vision_enabled boolean DEFAULT false NOT NULL,
    input_cost double precision,
    cached_input_cost double precision,
    output_cost double precision,
    max_output_tokens integer,
    cache_write_cost double precision DEFAULT 0.0,
    allowed_attachment_types text[] DEFAULT '{}'::text[] NOT NULL,
    ai_secret_id bigint,
    vision_llm_model_id bigint
);


--
-- Name: llm_models_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.llm_models_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: llm_models_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.llm_models_id_seq OWNED BY public.llm_models.id;


--
-- Name: llm_quota_usages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.llm_quota_usages (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    llm_quota_id bigint NOT NULL,
    input_tokens_used integer NOT NULL,
    output_tokens_used integer NOT NULL,
    usages integer NOT NULL,
    started_at timestamp(6) without time zone NOT NULL,
    reset_at timestamp(6) without time zone NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    cost_used numeric(20,10) DEFAULT 0.0 NOT NULL,
    cache_read_tokens_used integer DEFAULT 0 NOT NULL,
    cache_write_tokens_used integer DEFAULT 0 NOT NULL
);


--
-- Name: llm_quota_usages_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.llm_quota_usages_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: llm_quota_usages_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.llm_quota_usages_id_seq OWNED BY public.llm_quota_usages.id;


--
-- Name: llm_quotas; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.llm_quotas (
    id bigint NOT NULL,
    group_id bigint NOT NULL,
    llm_model_id bigint NOT NULL,
    max_tokens integer,
    max_usages integer,
    duration_seconds integer NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    max_cost numeric(20,10)
);


--
-- Name: llm_quotas_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.llm_quotas_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: llm_quotas_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.llm_quotas_id_seq OWNED BY public.llm_quotas.id;


--
-- Name: message_bus; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_bus (
    id integer NOT NULL,
    name character varying,
    context character varying,
    data text,
    created_at timestamp without time zone NOT NULL
);


--
-- Name: message_bus_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.message_bus_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: message_bus_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.message_bus_id_seq OWNED BY public.message_bus.id;


--
-- Name: model_accuracies; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.model_accuracies (
    id bigint NOT NULL,
    model character varying NOT NULL,
    classification_type character varying NOT NULL,
    flags_agreed integer DEFAULT 0 NOT NULL,
    flags_disagreed integer DEFAULT 0 NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: model_accuracies_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.model_accuracies_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: model_accuracies_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.model_accuracies_id_seq OWNED BY public.model_accuracies.id;


--
-- Name: moved_posts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.moved_posts (
    id bigint NOT NULL,
    old_topic_id bigint NOT NULL,
    old_post_id bigint NOT NULL,
    old_post_number bigint NOT NULL,
    new_topic_id bigint NOT NULL,
    new_topic_title character varying NOT NULL,
    new_post_id bigint NOT NULL,
    new_post_number bigint NOT NULL,
    created_new_topic boolean DEFAULT false NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    old_topic_title character varying,
    post_user_id integer,
    user_id integer,
    full_move boolean
);


--
-- Name: moved_posts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.moved_posts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: moved_posts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.moved_posts_id_seq OWNED BY public.moved_posts.id;


--
-- Name: muted_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.muted_users (
    id integer NOT NULL,
    user_id integer NOT NULL,
    muted_user_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: muted_users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.muted_users_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: muted_users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.muted_users_id_seq OWNED BY public.muted_users.id;


--
-- Name: nested_hot_post_scores; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.nested_hot_post_scores (
    post_id bigint NOT NULL,
    topic_id bigint NOT NULL,
    hot_score double precision NOT NULL,
    thread_hot_score double precision NOT NULL
);


--
-- Name: nested_hot_score_snapshots; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.nested_hot_score_snapshots (
    topic_id bigint NOT NULL,
    calculated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: nested_topics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.nested_topics (
    id bigint NOT NULL,
    topic_id bigint NOT NULL,
    pinned_post_ids bigint[] DEFAULT '{}'::bigint[] NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: nested_topics_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.nested_topics_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: nested_topics_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.nested_topics_id_seq OWNED BY public.nested_topics.id;


--
-- Name: nested_view_post_stats; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.nested_view_post_stats (
    id bigint NOT NULL,
    post_id bigint NOT NULL,
    direct_reply_count integer DEFAULT 0 NOT NULL,
    total_descendant_count integer DEFAULT 0 NOT NULL,
    whisper_direct_reply_count integer DEFAULT 0 NOT NULL,
    whisper_total_descendant_count integer DEFAULT 0 NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: nested_view_post_stats_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.nested_view_post_stats_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: nested_view_post_stats_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.nested_view_post_stats_id_seq OWNED BY public.nested_view_post_stats.id;


--
-- Name: notifications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.notifications (
    notification_type integer NOT NULL,
    user_id integer NOT NULL,
    data character varying(1000) NOT NULL,
    read boolean DEFAULT false NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    topic_id integer,
    post_number integer,
    post_action_id integer,
    high_priority boolean DEFAULT false NOT NULL,
    id bigint NOT NULL
);


--
-- Name: notifications_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.notifications_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: notifications_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.notifications_id_seq OWNED BY public.notifications.id;


--
-- Name: oauth2_user_infos; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.oauth2_user_infos (
    id integer NOT NULL,
    user_id integer NOT NULL,
    uid character varying NOT NULL,
    provider character varying NOT NULL,
    email character varying,
    name character varying,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: oauth2_user_infos_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.oauth2_user_infos_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: oauth2_user_infos_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.oauth2_user_infos_id_seq OWNED BY public.oauth2_user_infos.id;


--
-- Name: onceoff_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.onceoff_logs (
    id integer NOT NULL,
    job_name character varying,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: onceoff_logs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.onceoff_logs_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: onceoff_logs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.onceoff_logs_id_seq OWNED BY public.onceoff_logs.id;


--
-- Name: optimized_images; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.optimized_images (
    id integer NOT NULL,
    sha1 character varying(40) NOT NULL,
    extension character varying(10) NOT NULL,
    width integer NOT NULL,
    height integer NOT NULL,
    upload_id integer NOT NULL,
    url character varying NOT NULL,
    filesize integer,
    etag character varying,
    version integer,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: optimized_images_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.optimized_images_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: optimized_images_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.optimized_images_id_seq OWNED BY public.optimized_images.id;


--
-- Name: optimized_videos; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.optimized_videos (
    id bigint NOT NULL,
    upload_id integer NOT NULL,
    optimized_upload_id integer NOT NULL,
    adapter character varying,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: optimized_videos_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.optimized_videos_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: optimized_videos_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.optimized_videos_id_seq OWNED BY public.optimized_videos.id;


--
-- Name: permalinks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.permalinks (
    id integer NOT NULL,
    url character varying(1000) NOT NULL,
    topic_id integer,
    post_id integer,
    category_id integer,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    external_url character varying(1000),
    tag_id integer,
    user_id integer
);


--
-- Name: permalinks_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.permalinks_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: permalinks_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.permalinks_id_seq OWNED BY public.permalinks.id;


--
-- Name: plugin_store_rows; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.plugin_store_rows (
    id integer NOT NULL,
    plugin_name character varying NOT NULL,
    key character varying NOT NULL,
    type_name character varying NOT NULL,
    value text
);


--
-- Name: plugin_store_rows_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.plugin_store_rows_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: plugin_store_rows_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.plugin_store_rows_id_seq OWNED BY public.plugin_store_rows.id;


--
-- Name: policy_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.policy_users (
    id bigint NOT NULL,
    post_policy_id bigint NOT NULL,
    user_id integer NOT NULL,
    accepted_at timestamp without time zone,
    revoked_at timestamp without time zone,
    expired_at timestamp without time zone,
    version character varying,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: policy_users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.policy_users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: policy_users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.policy_users_id_seq OWNED BY public.policy_users.id;


--
-- Name: poll_options; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.poll_options (
    id bigint NOT NULL,
    poll_id bigint,
    digest character varying NOT NULL,
    html text NOT NULL,
    anonymous_votes integer,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: poll_options_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.poll_options_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: poll_options_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.poll_options_id_seq OWNED BY public.poll_options.id;


--
-- Name: poll_votes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.poll_votes (
    poll_id bigint,
    poll_option_id bigint,
    user_id bigint,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    rank integer DEFAULT 0 NOT NULL
);


--
-- Name: polls; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.polls (
    id bigint NOT NULL,
    post_id bigint,
    name character varying DEFAULT 'poll'::character varying NOT NULL,
    close_at timestamp without time zone,
    type integer DEFAULT 0 NOT NULL,
    status integer DEFAULT 0 NOT NULL,
    results integer DEFAULT 0 NOT NULL,
    visibility integer DEFAULT 0 NOT NULL,
    min integer,
    max integer,
    step integer,
    anonymous_voters integer,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    chart_type integer DEFAULT 0 NOT NULL,
    groups character varying,
    title character varying,
    dynamic boolean DEFAULT false NOT NULL,
    closed_by_id integer,
    closed_at timestamp(6) without time zone
);


--
-- Name: polls_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.polls_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: polls_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.polls_id_seq OWNED BY public.polls.id;


--
-- Name: post_action_types; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.post_action_types (
    name_key character varying(50) NOT NULL,
    is_flag boolean DEFAULT false NOT NULL,
    icon character varying(20),
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    id integer NOT NULL,
    "position" integer DEFAULT 0 NOT NULL,
    score_bonus double precision DEFAULT 0.0 NOT NULL,
    reviewable_priority integer DEFAULT 0 NOT NULL
);


--
-- Name: post_action_types_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.post_action_types_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: post_action_types_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.post_action_types_id_seq OWNED BY public.post_action_types.id;


--
-- Name: post_actions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.post_actions (
    id integer NOT NULL,
    post_id integer NOT NULL,
    user_id integer NOT NULL,
    post_action_type_id integer NOT NULL,
    deleted_at timestamp without time zone,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    deleted_by_id integer,
    related_post_id integer,
    staff_took_action boolean DEFAULT false NOT NULL,
    deferred_by_id integer,
    targets_topic boolean DEFAULT false NOT NULL,
    agreed_at timestamp without time zone,
    agreed_by_id integer,
    deferred_at timestamp without time zone,
    disagreed_at timestamp without time zone,
    disagreed_by_id integer
);


--
-- Name: post_actions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.post_actions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: post_actions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.post_actions_id_seq OWNED BY public.post_actions.id;


--
-- Name: post_custom_fields; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.post_custom_fields (
    id integer NOT NULL,
    post_id integer NOT NULL,
    name character varying(256) NOT NULL,
    value text,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: post_custom_fields_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.post_custom_fields_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: post_custom_fields_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.post_custom_fields_id_seq OWNED BY public.post_custom_fields.id;


--
-- Name: post_custom_prompts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.post_custom_prompts (
    id bigint NOT NULL,
    post_id integer NOT NULL,
    custom_prompt json NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: post_custom_prompts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.post_custom_prompts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: post_custom_prompts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.post_custom_prompts_id_seq OWNED BY public.post_custom_prompts.id;


--
-- Name: post_details; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.post_details (
    id integer NOT NULL,
    post_id integer,
    key character varying,
    value character varying,
    extra text,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: post_details_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.post_details_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: post_details_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.post_details_id_seq OWNED BY public.post_details.id;


--
-- Name: post_hotlinked_media; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.post_hotlinked_media (
    id bigint NOT NULL,
    post_id bigint NOT NULL,
    url character varying NOT NULL,
    status public.hotlinked_media_status NOT NULL,
    upload_id bigint,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: post_hotlinked_media_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.post_hotlinked_media_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: post_hotlinked_media_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.post_hotlinked_media_id_seq OWNED BY public.post_hotlinked_media.id;


--
-- Name: post_localizations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.post_localizations (
    id bigint NOT NULL,
    post_id integer NOT NULL,
    post_version integer NOT NULL,
    locale character varying(20) NOT NULL,
    raw text NOT NULL,
    cooked text NOT NULL,
    localizer_user_id integer NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: post_localizations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.post_localizations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: post_localizations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.post_localizations_id_seq OWNED BY public.post_localizations.id;


--
-- Name: post_policies; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.post_policies (
    id bigint NOT NULL,
    post_id bigint NOT NULL,
    renew_start timestamp without time zone,
    renew_days integer,
    next_renew_at timestamp without time zone,
    reminder character varying,
    last_reminded_at timestamp without time zone,
    version character varying,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    renew_interval integer,
    private boolean DEFAULT false NOT NULL,
    last_bumped_at timestamp(6) without time zone,
    add_users_to_group integer
);


--
-- Name: post_policies_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.post_policies_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: post_policies_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.post_policies_id_seq OWNED BY public.post_policies.id;


--
-- Name: post_policy_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.post_policy_groups (
    id bigint NOT NULL,
    group_id integer NOT NULL,
    post_policy_id bigint NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: post_policy_groups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.post_policy_groups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: post_policy_groups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.post_policy_groups_id_seq OWNED BY public.post_policy_groups.id;


--
-- Name: post_replies; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.post_replies (
    post_id integer,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    reply_post_id integer
);


--
-- Name: post_reply_keys; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.post_reply_keys (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    post_id integer NOT NULL,
    reply_key uuid NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: post_reply_keys_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.post_reply_keys_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: post_reply_keys_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.post_reply_keys_id_seq OWNED BY public.post_reply_keys.id;


--
-- Name: post_revisions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.post_revisions (
    id integer NOT NULL,
    user_id integer,
    post_id integer,
    modifications text,
    number integer,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    hidden boolean DEFAULT false NOT NULL
);


--
-- Name: post_revisions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.post_revisions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: post_revisions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.post_revisions_id_seq OWNED BY public.post_revisions.id;


--
-- Name: post_search_data; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.post_search_data (
    post_id integer NOT NULL,
    search_data tsvector,
    raw_data text,
    locale character varying,
    version integer DEFAULT 0,
    private_message boolean NOT NULL
);


--
-- Name: post_stats; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.post_stats (
    id integer NOT NULL,
    post_id integer,
    drafts_saved integer,
    typing_duration_msecs integer,
    composer_open_duration_msecs integer,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    composer_version integer,
    writing_device character varying,
    writing_device_user_agent character varying
);


--
-- Name: post_stats_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.post_stats_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: post_stats_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.post_stats_id_seq OWNED BY public.post_stats.id;


--
-- Name: post_timings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.post_timings (
    topic_id integer NOT NULL,
    post_number integer NOT NULL,
    user_id integer NOT NULL,
    msecs integer NOT NULL
);


--
-- Name: post_voting_comment_custom_fields; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.post_voting_comment_custom_fields (
    id bigint NOT NULL,
    post_voting_comment_id bigint NOT NULL,
    name character varying(256) NOT NULL,
    value text,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: post_voting_comment_custom_fields_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.post_voting_comment_custom_fields_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: post_voting_comment_custom_fields_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.post_voting_comment_custom_fields_id_seq OWNED BY public.post_voting_comment_custom_fields.id;


--
-- Name: post_voting_comments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.post_voting_comments (
    id bigint NOT NULL,
    post_id integer NOT NULL,
    user_id integer NOT NULL,
    raw text NOT NULL,
    cooked text NOT NULL,
    cooked_version integer,
    deleted_at timestamp without time zone,
    deleted_by_id integer,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    qa_vote_count integer DEFAULT 0,
    last_editor_id integer NOT NULL,
    CONSTRAINT qa_vote_count_positive CHECK ((qa_vote_count >= 0))
);


--
-- Name: post_voting_comments_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.post_voting_comments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: post_voting_comments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.post_voting_comments_id_seq OWNED BY public.post_voting_comments.id;


--
-- Name: post_voting_votes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.post_voting_votes (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    direction character varying NOT NULL,
    votable_type character varying NOT NULL,
    votable_id bigint NOT NULL
);


--
-- Name: post_voting_votes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.post_voting_votes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: post_voting_votes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.post_voting_votes_id_seq OWNED BY public.post_voting_votes.id;


--
-- Name: posts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.posts_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: posts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.posts_id_seq OWNED BY public.posts.id;


--
-- Name: problem_check_trackers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.problem_check_trackers (
    id bigint NOT NULL,
    identifier character varying NOT NULL,
    blips integer DEFAULT 0 NOT NULL,
    last_run_at timestamp(6) without time zone,
    next_run_at timestamp(6) without time zone,
    last_success_at timestamp(6) without time zone,
    last_problem_at timestamp(6) without time zone,
    details json DEFAULT '{}'::json,
    target character varying DEFAULT '__NULL__'::character varying NOT NULL,
    ignored_at timestamp(6) without time zone
);


--
-- Name: problem_check_trackers_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.problem_check_trackers_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: problem_check_trackers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.problem_check_trackers_id_seq OWNED BY public.problem_check_trackers.id;


--
-- Name: published_pages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.published_pages (
    id bigint NOT NULL,
    topic_id bigint NOT NULL,
    slug character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    public boolean DEFAULT false NOT NULL
);


--
-- Name: published_pages_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.published_pages_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: published_pages_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.published_pages_id_seq OWNED BY public.published_pages.id;


--
-- Name: push_subscriptions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.push_subscriptions (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    data character varying NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    error_count integer DEFAULT 0 NOT NULL,
    first_error_at timestamp without time zone
);


--
-- Name: push_subscriptions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.push_subscriptions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: push_subscriptions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.push_subscriptions_id_seq OWNED BY public.push_subscriptions.id;


--
-- Name: quoted_posts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.quoted_posts (
    id integer NOT NULL,
    post_id integer NOT NULL,
    quoted_post_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: quoted_posts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.quoted_posts_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: quoted_posts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.quoted_posts_id_seq OWNED BY public.quoted_posts.id;


--
-- Name: rag_document_fragments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rag_document_fragments (
    id bigint NOT NULL,
    fragment text NOT NULL,
    upload_id integer NOT NULL,
    fragment_number integer NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    metadata text,
    target_id bigint NOT NULL,
    target_type character varying(800) NOT NULL
);


--
-- Name: rag_document_fragments_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.rag_document_fragments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: rag_document_fragments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.rag_document_fragments_id_seq OWNED BY public.rag_document_fragments.id;


--
-- Name: rag_document_sources; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rag_document_sources (
    id bigint NOT NULL,
    target_type character varying(800) NOT NULL,
    target_id bigint NOT NULL,
    url character varying(2000) NOT NULL,
    url_digest character varying(64) NOT NULL,
    refresh_interval_hours integer DEFAULT 24 NOT NULL,
    upload_id integer,
    etag character varying,
    last_modified character varying,
    last_fetched_at timestamp(6) without time zone,
    next_refresh_at timestamp(6) without time zone,
    last_error_at timestamp(6) without time zone,
    last_error text,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    pending_upload_id integer,
    managed boolean DEFAULT false NOT NULL
);


--
-- Name: rag_document_sources_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.rag_document_sources_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: rag_document_sources_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.rag_document_sources_id_seq OWNED BY public.rag_document_sources.id;


--
-- Name: redelivering_webhook_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.redelivering_webhook_events (
    id bigint NOT NULL,
    web_hook_event_id bigint NOT NULL,
    processing boolean DEFAULT false NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: redelivering_webhook_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.redelivering_webhook_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: redelivering_webhook_events_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.redelivering_webhook_events_id_seq OWNED BY public.redelivering_webhook_events.id;


--
-- Name: remote_themes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.remote_themes (
    id integer NOT NULL,
    remote_url character varying NOT NULL,
    remote_version character varying,
    local_version character varying,
    about_url character varying,
    license_url character varying,
    commits_behind integer,
    remote_updated_at timestamp without time zone,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    private_key text,
    branch character varying,
    last_error_text text,
    authors character varying,
    theme_version character varying,
    minimum_discourse_version character varying,
    maximum_discourse_version character varying,
    local_compat_ref character varying,
    remote_compat_ref character varying
);


--
-- Name: remote_themes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.remote_themes_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: remote_themes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.remote_themes_id_seq OWNED BY public.remote_themes.id;


--
-- Name: reviewable_claimed_topics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.reviewable_claimed_topics (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    topic_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    automatic boolean DEFAULT false NOT NULL
);


--
-- Name: reviewable_claimed_topics_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.reviewable_claimed_topics_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: reviewable_claimed_topics_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.reviewable_claimed_topics_id_seq OWNED BY public.reviewable_claimed_topics.id;


--
-- Name: reviewable_histories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.reviewable_histories (
    id bigint NOT NULL,
    reviewable_id integer NOT NULL,
    reviewable_history_type integer NOT NULL,
    status integer NOT NULL,
    created_by_id integer NOT NULL,
    edited json,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: reviewable_histories_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.reviewable_histories_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: reviewable_histories_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.reviewable_histories_id_seq OWNED BY public.reviewable_histories.id;


--
-- Name: reviewable_notes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.reviewable_notes (
    id bigint NOT NULL,
    reviewable_id bigint NOT NULL,
    user_id bigint NOT NULL,
    content text NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: reviewable_notes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.reviewable_notes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: reviewable_notes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.reviewable_notes_id_seq OWNED BY public.reviewable_notes.id;


--
-- Name: reviewable_scores; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.reviewable_scores (
    id bigint NOT NULL,
    reviewable_id integer NOT NULL,
    user_id integer NOT NULL,
    reviewable_score_type integer NOT NULL,
    status integer NOT NULL,
    score double precision DEFAULT 0.0 NOT NULL,
    take_action_bonus double precision DEFAULT 0.0 NOT NULL,
    reviewed_by_id integer,
    reviewed_at timestamp without time zone,
    meta_topic_id integer,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    reason character varying,
    user_accuracy_bonus double precision DEFAULT 0.0 NOT NULL,
    context character varying
);


--
-- Name: reviewable_scores_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.reviewable_scores_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: reviewable_scores_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.reviewable_scores_id_seq OWNED BY public.reviewable_scores.id;


--
-- Name: reviewables; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.reviewables (
    id bigint NOT NULL,
    type character varying NOT NULL,
    status integer DEFAULT 0 NOT NULL,
    created_by_id integer NOT NULL,
    reviewable_by_moderator boolean DEFAULT false NOT NULL,
    reviewable_by_group_id integer,
    category_id integer,
    topic_id integer,
    score double precision DEFAULT 0.0 NOT NULL,
    potential_spam boolean DEFAULT false NOT NULL,
    target_id integer,
    target_type character varying,
    target_created_by_id integer,
    payload json,
    version integer DEFAULT 0 NOT NULL,
    latest_score timestamp without time zone,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    force_review boolean DEFAULT false NOT NULL,
    reject_reason text,
    potentially_illegal boolean DEFAULT false,
    type_source character varying DEFAULT 'unknown'::character varying NOT NULL
);


--
-- Name: reviewables_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.reviewables_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: reviewables_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.reviewables_id_seq OWNED BY public.reviewables.id;


--
-- Name: scheduler_stats; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.scheduler_stats (
    id integer NOT NULL,
    name character varying NOT NULL,
    hostname character varying NOT NULL,
    pid integer NOT NULL,
    duration_ms integer,
    live_slots_start integer,
    live_slots_finish integer,
    started_at timestamp without time zone NOT NULL,
    success boolean,
    error text
);


--
-- Name: scheduler_stats_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.scheduler_stats_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: scheduler_stats_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.scheduler_stats_id_seq OWNED BY public.scheduler_stats.id;


--
-- Name: schema_migration_details; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migration_details (
    id integer NOT NULL,
    version character varying NOT NULL,
    name character varying,
    hostname character varying,
    git_version character varying,
    rails_version character varying,
    duration integer,
    direction character varying,
    created_at timestamp without time zone NOT NULL
);


--
-- Name: schema_migration_details_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.schema_migration_details_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: schema_migration_details_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.schema_migration_details_id_seq OWNED BY public.schema_migration_details.id;


--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version character varying NOT NULL
);


--
-- Name: screened_emails; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.screened_emails (
    id integer NOT NULL,
    email character varying NOT NULL,
    action_type integer NOT NULL,
    match_count integer DEFAULT 0 NOT NULL,
    last_match_at timestamp without time zone,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    ip_address inet
);


--
-- Name: screened_emails_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.screened_emails_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: screened_emails_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.screened_emails_id_seq OWNED BY public.screened_emails.id;


--
-- Name: screened_ip_addresses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.screened_ip_addresses (
    id integer NOT NULL,
    ip_address inet NOT NULL,
    action_type integer NOT NULL,
    match_count integer DEFAULT 0 NOT NULL,
    last_match_at timestamp without time zone,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: screened_ip_addresses_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.screened_ip_addresses_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: screened_ip_addresses_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.screened_ip_addresses_id_seq OWNED BY public.screened_ip_addresses.id;


--
-- Name: screened_urls; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.screened_urls (
    id integer NOT NULL,
    url character varying NOT NULL,
    domain character varying NOT NULL,
    action_type integer NOT NULL,
    match_count integer DEFAULT 0 NOT NULL,
    last_match_at timestamp without time zone,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    ip_address inet
);


--
-- Name: screened_urls_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.screened_urls_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: screened_urls_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.screened_urls_id_seq OWNED BY public.screened_urls.id;


--
-- Name: search_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.search_logs (
    id integer NOT NULL,
    term character varying NOT NULL,
    user_id integer,
    ip_address inet,
    search_result_id integer,
    search_type integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    search_result_type integer,
    user_agent character varying(2000),
    crawler boolean DEFAULT false NOT NULL,
    likely_crawler boolean DEFAULT false NOT NULL,
    session_id character varying(32)
);


--
-- Name: search_logs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.search_logs_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: search_logs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.search_logs_id_seq OWNED BY public.search_logs.id;


--
-- Name: shared_ai_conversations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.shared_ai_conversations (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    target_id integer NOT NULL,
    target_type character varying NOT NULL,
    title character varying NOT NULL,
    llm_name character varying NOT NULL,
    context jsonb NOT NULL,
    share_key character varying NOT NULL,
    excerpt character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: shared_ai_conversations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.shared_ai_conversations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: shared_ai_conversations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.shared_ai_conversations_id_seq OWNED BY public.shared_ai_conversations.id;


--
-- Name: shared_drafts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.shared_drafts (
    topic_id integer NOT NULL,
    category_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    id bigint NOT NULL
);


--
-- Name: shared_drafts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.shared_drafts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: shared_drafts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.shared_drafts_id_seq OWNED BY public.shared_drafts.id;


--
-- Name: shelved_notifications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.shelved_notifications (
    id bigint NOT NULL,
    notification_id bigint NOT NULL
);


--
-- Name: shelved_notifications_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.shelved_notifications_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: shelved_notifications_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.shelved_notifications_id_seq OWNED BY public.shelved_notifications.id;


--
-- Name: sidebar_section_links; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sidebar_section_links (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    linkable_id integer NOT NULL,
    linkable_type character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    sidebar_section_id integer,
    "position" integer DEFAULT 0 NOT NULL
);


--
-- Name: sidebar_section_links_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.sidebar_section_links_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: sidebar_section_links_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.sidebar_section_links_id_seq OWNED BY public.sidebar_section_links.id;


--
-- Name: sidebar_section_localizations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sidebar_section_localizations (
    id bigint NOT NULL,
    sidebar_section_id bigint NOT NULL,
    locale character varying(20) NOT NULL,
    title character varying(30) NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: sidebar_section_localizations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.sidebar_section_localizations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: sidebar_section_localizations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.sidebar_section_localizations_id_seq OWNED BY public.sidebar_section_localizations.id;


--
-- Name: sidebar_sections; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sidebar_sections (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    title character varying(30) NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    public boolean DEFAULT false NOT NULL,
    section_type integer,
    locale character varying(20)
);


--
-- Name: sidebar_sections_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.sidebar_sections_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: sidebar_sections_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.sidebar_sections_id_seq OWNED BY public.sidebar_sections.id;


--
-- Name: sidebar_url_localizations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sidebar_url_localizations (
    id bigint NOT NULL,
    sidebar_url_id bigint NOT NULL,
    locale character varying(20) NOT NULL,
    name character varying(80) NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: sidebar_url_localizations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.sidebar_url_localizations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: sidebar_url_localizations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.sidebar_url_localizations_id_seq OWNED BY public.sidebar_url_localizations.id;


--
-- Name: sidebar_urls; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sidebar_urls (
    id bigint NOT NULL,
    name character varying(80) NOT NULL,
    value character varying(1000) NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    icon character varying(40) NOT NULL,
    external boolean DEFAULT false NOT NULL,
    segment integer DEFAULT 0 NOT NULL,
    locale character varying(20)
);


--
-- Name: sidebar_urls_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.sidebar_urls_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: sidebar_urls_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.sidebar_urls_id_seq OWNED BY public.sidebar_urls.id;


--
-- Name: silenced_assignments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.silenced_assignments (
    id bigint NOT NULL,
    assignment_id bigint NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: silenced_assignments_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.silenced_assignments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: silenced_assignments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.silenced_assignments_id_seq OWNED BY public.silenced_assignments.id;


--
-- Name: single_sign_on_records; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.single_sign_on_records (
    id integer NOT NULL,
    user_id integer NOT NULL,
    external_id character varying NOT NULL,
    last_payload text NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    external_username character varying,
    external_email character varying,
    external_name character varying,
    external_avatar_url character varying(2000),
    external_profile_background_url character varying,
    external_card_background_url character varying
);


--
-- Name: single_sign_on_records_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.single_sign_on_records_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: single_sign_on_records_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.single_sign_on_records_id_seq OWNED BY public.single_sign_on_records.id;


--
-- Name: site_setting_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.site_setting_groups (
    id bigint NOT NULL,
    name character varying NOT NULL,
    group_ids character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: site_setting_groups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.site_setting_groups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: site_setting_groups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.site_setting_groups_id_seq OWNED BY public.site_setting_groups.id;


--
-- Name: site_setting_localizations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.site_setting_localizations (
    id bigint NOT NULL,
    setting_name character varying NOT NULL,
    locale character varying(20) NOT NULL,
    value text NOT NULL,
    cooked text,
    localizer_user_id integer,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: site_setting_localizations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.site_setting_localizations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: site_setting_localizations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.site_setting_localizations_id_seq OWNED BY public.site_setting_localizations.id;


--
-- Name: site_settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.site_settings (
    id integer NOT NULL,
    name character varying NOT NULL,
    data_type integer NOT NULL,
    value text,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: site_settings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.site_settings_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: site_settings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.site_settings_id_seq OWNED BY public.site_settings.id;


--
-- Name: sitemaps; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sitemaps (
    id bigint NOT NULL,
    name character varying NOT NULL,
    last_posted_at timestamp without time zone NOT NULL,
    enabled boolean DEFAULT true NOT NULL
);


--
-- Name: sitemaps_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.sitemaps_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: sitemaps_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.sitemaps_id_seq OWNED BY public.sitemaps.id;


--
-- Name: skipped_email_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.skipped_email_logs (
    id bigint NOT NULL,
    email_type character varying NOT NULL,
    to_address character varying NOT NULL,
    user_id integer,
    post_id integer,
    reason_type integer NOT NULL,
    custom_reason text,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: skipped_email_logs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.skipped_email_logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: skipped_email_logs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.skipped_email_logs_id_seq OWNED BY public.skipped_email_logs.id;


--
-- Name: stylesheet_cache; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.stylesheet_cache (
    id integer NOT NULL,
    target character varying NOT NULL,
    digest character varying NOT NULL,
    content text NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    theme_id integer DEFAULT '-1'::integer NOT NULL,
    source_map text
);


--
-- Name: stylesheet_cache_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.stylesheet_cache_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: stylesheet_cache_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.stylesheet_cache_id_seq OWNED BY public.stylesheet_cache.id;


--
-- Name: summary_sections; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.summary_sections (
    id bigint NOT NULL,
    target_id integer NOT NULL,
    target_type character varying NOT NULL,
    content_range int4range,
    summarized_text character varying NOT NULL,
    meta_section_id integer,
    original_content_sha character varying NOT NULL,
    algorithm character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: summary_sections_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.summary_sections_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: summary_sections_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.summary_sections_id_seq OWNED BY public.summary_sections.id;


--
-- Name: tag_group_memberships; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tag_group_memberships (
    id integer NOT NULL,
    tag_id integer NOT NULL,
    tag_group_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: tag_group_memberships_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.tag_group_memberships_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: tag_group_memberships_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.tag_group_memberships_id_seq OWNED BY public.tag_group_memberships.id;


--
-- Name: tag_group_permissions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tag_group_permissions (
    id bigint NOT NULL,
    tag_group_id bigint NOT NULL,
    group_id bigint NOT NULL,
    permission_type integer DEFAULT 1 NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: tag_group_permissions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.tag_group_permissions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: tag_group_permissions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.tag_group_permissions_id_seq OWNED BY public.tag_group_permissions.id;


--
-- Name: tag_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tag_groups (
    id integer NOT NULL,
    name character varying(100) NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    parent_tag_id integer,
    one_per_topic boolean DEFAULT false
);


--
-- Name: tag_groups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.tag_groups_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: tag_groups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.tag_groups_id_seq OWNED BY public.tag_groups.id;


--
-- Name: tag_localizations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tag_localizations (
    id bigint NOT NULL,
    tag_id bigint NOT NULL,
    locale character varying(20) NOT NULL,
    name character varying NOT NULL,
    description character varying(1000),
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    description_cooked character varying(2000),
    description_cooked_version integer
);


--
-- Name: tag_localizations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.tag_localizations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: tag_localizations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.tag_localizations_id_seq OWNED BY public.tag_localizations.id;


--
-- Name: tag_search_data; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tag_search_data (
    tag_id integer NOT NULL,
    search_data tsvector,
    raw_data text,
    locale text,
    version integer DEFAULT 0
);


--
-- Name: tag_search_data_tag_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.tag_search_data_tag_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: tag_search_data_tag_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.tag_search_data_tag_id_seq OWNED BY public.tag_search_data.tag_id;


--
-- Name: tag_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tag_users (
    id integer NOT NULL,
    tag_id integer NOT NULL,
    user_id integer NOT NULL,
    notification_level integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: tag_users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.tag_users_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: tag_users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.tag_users_id_seq OWNED BY public.tag_users.id;


--
-- Name: tags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tags (
    id integer NOT NULL,
    name character varying NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    pm_topic_count integer DEFAULT 0 NOT NULL,
    target_tag_id integer,
    description character varying(1000),
    public_topic_count integer DEFAULT 0 NOT NULL,
    staff_topic_count integer DEFAULT 0 NOT NULL,
    locale character varying(20),
    slug character varying DEFAULT ''::character varying NOT NULL,
    description_cooked character varying(2000),
    description_cooked_version integer
);


--
-- Name: tags_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.tags_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: tags_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.tags_id_seq OWNED BY public.tags.id;


--
-- Name: tags_web_hooks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tags_web_hooks (
    web_hook_id bigint NOT NULL,
    tag_id bigint NOT NULL
);


--
-- Name: theme_fields; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.theme_fields (
    id integer NOT NULL,
    theme_id integer NOT NULL,
    target_id integer NOT NULL,
    name character varying(255) NOT NULL,
    value text NOT NULL,
    value_baked text,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    compiler_version character varying(50) DEFAULT 0 NOT NULL,
    error character varying,
    upload_id integer,
    type_id integer DEFAULT 0 NOT NULL
);


--
-- Name: theme_fields_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.theme_fields_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: theme_fields_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.theme_fields_id_seq OWNED BY public.theme_fields.id;


--
-- Name: theme_modifier_sets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.theme_modifier_sets (
    id bigint NOT NULL,
    theme_id bigint NOT NULL,
    serialize_topic_excerpts boolean,
    csp_extensions character varying[],
    svg_icons character varying[],
    topic_thumbnail_sizes character varying[],
    custom_homepage boolean,
    serialize_post_user_badges character varying[],
    theme_setting_modifiers jsonb,
    serialize_topic_op_likes_data boolean,
    serialize_topic_is_hot boolean,
    only_theme_color_schemes boolean
);


--
-- Name: theme_modifier_sets_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.theme_modifier_sets_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: theme_modifier_sets_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.theme_modifier_sets_id_seq OWNED BY public.theme_modifier_sets.id;


--
-- Name: theme_settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.theme_settings (
    id bigint NOT NULL,
    name character varying(255) NOT NULL,
    data_type integer NOT NULL,
    value text,
    theme_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    json_value jsonb
);


--
-- Name: theme_settings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.theme_settings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: theme_settings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.theme_settings_id_seq OWNED BY public.theme_settings.id;


--
-- Name: theme_settings_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.theme_settings_migrations (
    id bigint NOT NULL,
    theme_id integer NOT NULL,
    theme_field_id integer NOT NULL,
    version integer NOT NULL,
    name character varying(150) NOT NULL,
    diff json NOT NULL,
    created_at timestamp(6) without time zone NOT NULL
);


--
-- Name: theme_settings_migrations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.theme_settings_migrations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: theme_settings_migrations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.theme_settings_migrations_id_seq OWNED BY public.theme_settings_migrations.id;


--
-- Name: theme_site_settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.theme_site_settings (
    id bigint NOT NULL,
    theme_id integer NOT NULL,
    name character varying NOT NULL,
    data_type integer NOT NULL,
    value text,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: theme_site_settings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.theme_site_settings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: theme_site_settings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.theme_site_settings_id_seq OWNED BY public.theme_site_settings.id;


--
-- Name: theme_svg_sprites; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.theme_svg_sprites (
    id bigint NOT NULL,
    theme_id integer NOT NULL,
    upload_id integer NOT NULL,
    sprite bytea NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: theme_svg_sprites_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.theme_svg_sprites_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: theme_svg_sprites_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.theme_svg_sprites_id_seq OWNED BY public.theme_svg_sprites.id;


--
-- Name: theme_translation_overrides; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.theme_translation_overrides (
    id bigint NOT NULL,
    theme_id integer NOT NULL,
    locale character varying NOT NULL,
    translation_key character varying NOT NULL,
    value character varying NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: theme_translation_overrides_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.theme_translation_overrides_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: theme_translation_overrides_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.theme_translation_overrides_id_seq OWNED BY public.theme_translation_overrides.id;


--
-- Name: themes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.themes (
    id integer NOT NULL,
    name character varying NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    compiler_version integer DEFAULT 0 NOT NULL,
    user_selectable boolean DEFAULT false NOT NULL,
    hidden boolean DEFAULT false NOT NULL,
    color_scheme_id integer,
    remote_theme_id integer,
    component boolean DEFAULT false NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    auto_update boolean DEFAULT true NOT NULL,
    dark_color_scheme_id integer
);


--
-- Name: themes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.themes_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: themes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.themes_id_seq OWNED BY public.themes.id;


--
-- Name: top_topics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.top_topics (
    id integer NOT NULL,
    topic_id integer,
    yearly_posts_count integer DEFAULT 0 NOT NULL,
    yearly_views_count integer DEFAULT 0 NOT NULL,
    yearly_likes_count integer DEFAULT 0 NOT NULL,
    monthly_posts_count integer DEFAULT 0 NOT NULL,
    monthly_views_count integer DEFAULT 0 NOT NULL,
    monthly_likes_count integer DEFAULT 0 NOT NULL,
    weekly_posts_count integer DEFAULT 0 NOT NULL,
    weekly_views_count integer DEFAULT 0 NOT NULL,
    weekly_likes_count integer DEFAULT 0 NOT NULL,
    daily_posts_count integer DEFAULT 0 NOT NULL,
    daily_views_count integer DEFAULT 0 NOT NULL,
    daily_likes_count integer DEFAULT 0 NOT NULL,
    daily_score double precision DEFAULT 0.0,
    weekly_score double precision DEFAULT 0.0,
    monthly_score double precision DEFAULT 0.0,
    yearly_score double precision DEFAULT 0.0,
    all_score double precision DEFAULT 0.0,
    daily_op_likes_count integer DEFAULT 0 NOT NULL,
    weekly_op_likes_count integer DEFAULT 0 NOT NULL,
    monthly_op_likes_count integer DEFAULT 0 NOT NULL,
    yearly_op_likes_count integer DEFAULT 0 NOT NULL,
    quarterly_posts_count integer DEFAULT 0 NOT NULL,
    quarterly_views_count integer DEFAULT 0 NOT NULL,
    quarterly_likes_count integer DEFAULT 0 NOT NULL,
    quarterly_score double precision DEFAULT 0.0,
    quarterly_op_likes_count integer DEFAULT 0 NOT NULL
);


--
-- Name: top_topics_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.top_topics_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: top_topics_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.top_topics_id_seq OWNED BY public.top_topics.id;


--
-- Name: topic_allowed_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.topic_allowed_groups (
    id integer NOT NULL,
    group_id integer NOT NULL,
    topic_id integer NOT NULL
);


--
-- Name: topic_allowed_groups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.topic_allowed_groups_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: topic_allowed_groups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.topic_allowed_groups_id_seq OWNED BY public.topic_allowed_groups.id;


--
-- Name: topic_allowed_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.topic_allowed_users (
    id integer NOT NULL,
    user_id integer NOT NULL,
    topic_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: topic_allowed_users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.topic_allowed_users_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: topic_allowed_users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.topic_allowed_users_id_seq OWNED BY public.topic_allowed_users.id;


--
-- Name: topic_custom_fields; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.topic_custom_fields (
    id integer NOT NULL,
    topic_id integer NOT NULL,
    name character varying(256) NOT NULL,
    value text,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: topic_custom_fields_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.topic_custom_fields_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: topic_custom_fields_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.topic_custom_fields_id_seq OWNED BY public.topic_custom_fields.id;


--
-- Name: topic_embeds; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.topic_embeds (
    id integer NOT NULL,
    topic_id integer NOT NULL,
    post_id integer NOT NULL,
    embed_url character varying(1000) NOT NULL,
    content_sha1 character varying(40),
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    deleted_at timestamp without time zone,
    deleted_by_id integer,
    embed_content_cache text,
    content_truncated boolean
);


--
-- Name: topic_embeds_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.topic_embeds_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: topic_embeds_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.topic_embeds_id_seq OWNED BY public.topic_embeds.id;


--
-- Name: topic_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.topic_groups (
    id bigint NOT NULL,
    group_id integer NOT NULL,
    topic_id integer NOT NULL,
    last_read_post_number integer DEFAULT 0 NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: topic_groups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.topic_groups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: topic_groups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.topic_groups_id_seq OWNED BY public.topic_groups.id;


--
-- Name: topic_hot_scores; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.topic_hot_scores (
    id bigint NOT NULL,
    topic_id integer NOT NULL,
    score double precision DEFAULT 0.0 NOT NULL,
    recent_likes integer DEFAULT 0 NOT NULL,
    recent_posters integer DEFAULT 0 NOT NULL,
    recent_first_bumped_at timestamp(6) without time zone,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: topic_hot_scores_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.topic_hot_scores_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: topic_hot_scores_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.topic_hot_scores_id_seq OWNED BY public.topic_hot_scores.id;


--
-- Name: topic_invites; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.topic_invites (
    id integer NOT NULL,
    topic_id integer NOT NULL,
    invite_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: topic_invites_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.topic_invites_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: topic_invites_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.topic_invites_id_seq OWNED BY public.topic_invites.id;


--
-- Name: topic_link_clicks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.topic_link_clicks (
    id integer NOT NULL,
    topic_link_id integer NOT NULL,
    user_id integer,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    ip_address inet
);


--
-- Name: topic_link_clicks_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.topic_link_clicks_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: topic_link_clicks_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.topic_link_clicks_id_seq OWNED BY public.topic_link_clicks.id;


--
-- Name: topic_links; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.topic_links (
    id integer NOT NULL,
    topic_id integer NOT NULL,
    post_id integer,
    user_id integer NOT NULL,
    url character varying NOT NULL,
    domain character varying(100) NOT NULL,
    internal boolean DEFAULT false NOT NULL,
    link_topic_id integer,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    reflection boolean DEFAULT false,
    clicks integer DEFAULT 0 NOT NULL,
    link_post_id integer,
    title character varying,
    crawled_at timestamp without time zone,
    quote boolean DEFAULT false NOT NULL,
    extension character varying(10)
);


--
-- Name: topic_links_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.topic_links_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: topic_links_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.topic_links_id_seq OWNED BY public.topic_links.id;


--
-- Name: topic_localizations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.topic_localizations (
    id bigint NOT NULL,
    topic_id integer NOT NULL,
    locale character varying(20) NOT NULL,
    title character varying NOT NULL,
    fancy_title character varying NOT NULL,
    localizer_user_id integer NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    excerpt character varying
);


--
-- Name: topic_localizations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.topic_localizations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: topic_localizations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.topic_localizations_id_seq OWNED BY public.topic_localizations.id;


--
-- Name: topic_search_data; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.topic_search_data (
    topic_id integer NOT NULL,
    raw_data text,
    locale character varying NOT NULL,
    search_data tsvector,
    version integer DEFAULT 0
);


--
-- Name: topic_search_data_topic_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.topic_search_data_topic_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: topic_search_data_topic_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.topic_search_data_topic_id_seq OWNED BY public.topic_search_data.topic_id;


--
-- Name: topic_tags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.topic_tags (
    id integer NOT NULL,
    topic_id integer NOT NULL,
    tag_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: topic_tags_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.topic_tags_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: topic_tags_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.topic_tags_id_seq OWNED BY public.topic_tags.id;


--
-- Name: topic_thumbnails; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.topic_thumbnails (
    id bigint NOT NULL,
    upload_id bigint NOT NULL,
    optimized_image_id bigint,
    max_width integer NOT NULL,
    max_height integer NOT NULL
);


--
-- Name: topic_thumbnails_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.topic_thumbnails_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: topic_thumbnails_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.topic_thumbnails_id_seq OWNED BY public.topic_thumbnails.id;


--
-- Name: topic_timers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.topic_timers (
    id integer NOT NULL,
    execute_at timestamp without time zone NOT NULL,
    status_type integer NOT NULL,
    user_id integer NOT NULL,
    topic_id integer,
    based_on_last_post boolean DEFAULT false NOT NULL,
    deleted_at timestamp without time zone,
    deleted_by_id integer,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    category_id integer,
    public_type boolean DEFAULT true,
    duration_minutes integer,
    type character varying DEFAULT 'TopicTimer'::character varying NOT NULL,
    timerable_id integer NOT NULL
);


--
-- Name: topic_timers_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.topic_timers_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: topic_timers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.topic_timers_id_seq OWNED BY public.topic_timers.id;


--
-- Name: topic_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.topic_users (
    user_id integer NOT NULL,
    topic_id integer NOT NULL,
    posted boolean DEFAULT false NOT NULL,
    last_read_post_number integer,
    last_visited_at timestamp without time zone,
    first_visited_at timestamp without time zone,
    notification_level integer DEFAULT 1 NOT NULL,
    notifications_changed_at timestamp without time zone,
    notifications_reason_id integer,
    total_msecs_viewed integer DEFAULT 0 NOT NULL,
    cleared_pinned_at timestamp without time zone,
    id integer NOT NULL,
    last_emailed_post_number integer,
    liked boolean DEFAULT false,
    bookmarked boolean DEFAULT false,
    last_posted_at timestamp without time zone
);


--
-- Name: topic_users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.topic_users_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: topic_users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.topic_users_id_seq OWNED BY public.topic_users.id;


--
-- Name: topic_view_stats; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.topic_view_stats (
    id bigint NOT NULL,
    topic_id integer NOT NULL,
    viewed_at date NOT NULL,
    anonymous_views integer DEFAULT 0 NOT NULL,
    logged_in_views integer DEFAULT 0 NOT NULL
);


--
-- Name: topic_view_stats_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.topic_view_stats_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: topic_view_stats_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.topic_view_stats_id_seq OWNED BY public.topic_view_stats.id;


--
-- Name: topic_views; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.topic_views (
    topic_id integer NOT NULL,
    viewed_at date NOT NULL,
    user_id integer,
    ip_address inet
);


--
-- Name: topic_voting_category_settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.topic_voting_category_settings (
    id bigint NOT NULL,
    category_id integer,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: topic_voting_category_settings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.topic_voting_category_settings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: topic_voting_category_settings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.topic_voting_category_settings_id_seq OWNED BY public.topic_voting_category_settings.id;


--
-- Name: topic_voting_topic_vote_count; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.topic_voting_topic_vote_count (
    id bigint NOT NULL,
    topic_id integer,
    votes_count integer,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: topic_voting_topic_vote_count_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.topic_voting_topic_vote_count_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: topic_voting_topic_vote_count_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.topic_voting_topic_vote_count_id_seq OWNED BY public.topic_voting_topic_vote_count.id;


--
-- Name: topic_voting_votes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.topic_voting_votes (
    id bigint NOT NULL,
    topic_id integer,
    user_id integer,
    archive boolean DEFAULT false,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: topic_voting_votes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.topic_voting_votes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: topic_voting_votes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.topic_voting_votes_id_seq OWNED BY public.topic_voting_votes.id;


--
-- Name: topics_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.topics_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: topics_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.topics_id_seq OWNED BY public.topics.id;


--
-- Name: translation_overrides; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.translation_overrides (
    id integer NOT NULL,
    locale character varying NOT NULL,
    translation_key character varying NOT NULL,
    value character varying NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    original_translation text,
    status integer DEFAULT 0 NOT NULL
);


--
-- Name: translation_overrides_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.translation_overrides_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: translation_overrides_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.translation_overrides_id_seq OWNED BY public.translation_overrides.id;


--
-- Name: unsubscribe_keys; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.unsubscribe_keys (
    key character varying(64) NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    unsubscribe_key_type character varying,
    topic_id integer,
    post_id integer
);


--
-- Name: upcoming_change_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.upcoming_change_events (
    id bigint NOT NULL,
    event_type integer NOT NULL,
    upcoming_change_name character varying NOT NULL,
    event_data json,
    acting_user_id bigint,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: upcoming_change_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.upcoming_change_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: upcoming_change_events_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.upcoming_change_events_id_seq OWNED BY public.upcoming_change_events.id;


--
-- Name: upload_references; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.upload_references (
    id bigint NOT NULL,
    upload_id bigint NOT NULL,
    target_type character varying NOT NULL,
    target_id bigint NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: upload_references_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.upload_references_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: upload_references_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.upload_references_id_seq OWNED BY public.upload_references.id;


--
-- Name: uploads; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.uploads (
    id integer NOT NULL,
    user_id integer NOT NULL,
    original_filename character varying NOT NULL,
    filesize bigint NOT NULL,
    width integer,
    height integer,
    url character varying NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    sha1 character varying(40),
    origin character varying(2000),
    retain_hours integer,
    extension character varying(10),
    thumbnail_width integer,
    thumbnail_height integer,
    etag character varying,
    secure boolean DEFAULT false NOT NULL,
    access_control_post_id bigint,
    original_sha1 character varying,
    animated boolean,
    verification_status integer DEFAULT 1 NOT NULL,
    security_last_changed_at timestamp without time zone,
    security_last_changed_reason character varying,
    dominant_color text
);


--
-- Name: uploads_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.uploads_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: uploads_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.uploads_id_seq OWNED BY public.uploads.id;


--
-- Name: user_actions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_actions (
    id integer NOT NULL,
    action_type integer NOT NULL,
    user_id integer NOT NULL,
    target_topic_id integer,
    target_post_id integer,
    target_user_id integer,
    acting_user_id integer,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: user_actions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_actions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_actions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_actions_id_seq OWNED BY public.user_actions.id;


--
-- Name: user_api_key_client_scopes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_api_key_client_scopes (
    id bigint NOT NULL,
    user_api_key_client_id bigint NOT NULL,
    name character varying(100) NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: user_api_key_client_scopes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_api_key_client_scopes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_api_key_client_scopes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_api_key_client_scopes_id_seq OWNED BY public.user_api_key_client_scopes.id;


--
-- Name: user_api_key_clients; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_api_key_clients (
    id bigint NOT NULL,
    client_id character varying NOT NULL,
    application_name character varying NOT NULL,
    public_key character varying,
    auth_redirect character varying,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: user_api_key_clients_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_api_key_clients_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_api_key_clients_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_api_key_clients_id_seq OWNED BY public.user_api_key_clients.id;


--
-- Name: user_api_key_scopes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_api_key_scopes (
    id bigint NOT NULL,
    user_api_key_id integer NOT NULL,
    name character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    allowed_parameters jsonb
);


--
-- Name: user_api_key_scopes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_api_key_scopes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_api_key_scopes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_api_key_scopes_id_seq OWNED BY public.user_api_key_scopes.id;


--
-- Name: user_api_keys; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_api_keys (
    id integer NOT NULL,
    user_id integer NOT NULL,
    client_id character varying,
    application_name character varying,
    push_url character varying,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    revoked_at timestamp without time zone,
    last_used_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    key_hash character varying NOT NULL,
    user_api_key_client_id bigint,
    expires_at timestamp(6) without time zone
);


--
-- Name: user_api_keys_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_api_keys_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_api_keys_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_api_keys_id_seq OWNED BY public.user_api_keys.id;


--
-- Name: user_archived_messages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_archived_messages (
    id integer NOT NULL,
    user_id integer NOT NULL,
    topic_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: user_archived_messages_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_archived_messages_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_archived_messages_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_archived_messages_id_seq OWNED BY public.user_archived_messages.id;


--
-- Name: user_associated_accounts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_associated_accounts (
    id bigint NOT NULL,
    provider_name character varying NOT NULL,
    provider_uid character varying NOT NULL,
    user_id integer,
    last_used timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    info jsonb DEFAULT '{}'::jsonb NOT NULL,
    credentials jsonb DEFAULT '{}'::jsonb NOT NULL,
    extra jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: user_associated_accounts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_associated_accounts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_associated_accounts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_associated_accounts_id_seq OWNED BY public.user_associated_accounts.id;


--
-- Name: user_associated_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_associated_groups (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    associated_group_id bigint NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: user_associated_groups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_associated_groups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_associated_groups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_associated_groups_id_seq OWNED BY public.user_associated_groups.id;


--
-- Name: user_auth_token_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_auth_token_logs (
    id integer NOT NULL,
    action character varying NOT NULL,
    user_auth_token_id integer,
    user_id integer,
    client_ip inet,
    user_agent character varying,
    auth_token character varying,
    created_at timestamp without time zone,
    path character varying
);


--
-- Name: user_auth_token_logs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_auth_token_logs_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_auth_token_logs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_auth_token_logs_id_seq OWNED BY public.user_auth_token_logs.id;


--
-- Name: user_auth_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_auth_tokens (
    id integer NOT NULL,
    user_id integer NOT NULL,
    auth_token character varying NOT NULL,
    prev_auth_token character varying NOT NULL,
    user_agent character varying,
    auth_token_seen boolean DEFAULT false NOT NULL,
    client_ip inet,
    rotated_at timestamp without time zone NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    seen_at timestamp without time zone,
    authenticated_with_oauth boolean DEFAULT false,
    impersonated_user_id integer,
    impersonation_expires_at timestamp(6) without time zone
);


--
-- Name: user_auth_tokens_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_auth_tokens_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_auth_tokens_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_auth_tokens_id_seq OWNED BY public.user_auth_tokens.id;


--
-- Name: user_avatars; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_avatars (
    id integer NOT NULL,
    user_id integer NOT NULL,
    custom_upload_id integer,
    gravatar_upload_id integer,
    last_gravatar_download_attempt timestamp without time zone,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: user_avatars_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_avatars_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_avatars_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_avatars_id_seq OWNED BY public.user_avatars.id;


--
-- Name: user_badges; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_badges (
    id integer NOT NULL,
    badge_id integer NOT NULL,
    user_id integer NOT NULL,
    granted_at timestamp without time zone NOT NULL,
    granted_by_id integer NOT NULL,
    post_id integer,
    seq integer DEFAULT 0 NOT NULL,
    featured_rank integer,
    created_at timestamp without time zone NOT NULL,
    is_favorite boolean,
    notification_id bigint
);


--
-- Name: user_badges_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_badges_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_badges_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_badges_id_seq OWNED BY public.user_badges.id;


--
-- Name: user_chat_channel_memberships; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_chat_channel_memberships (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    chat_channel_id bigint NOT NULL,
    last_read_message_id bigint,
    following boolean DEFAULT false NOT NULL,
    muted boolean DEFAULT false NOT NULL,
    desktop_notification_level integer DEFAULT 1 NOT NULL,
    mobile_notification_level integer DEFAULT 1 NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    last_unread_mention_when_emailed_id bigint,
    join_mode integer DEFAULT 0 NOT NULL,
    last_viewed_at timestamp(6) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    notification_level integer DEFAULT 1 NOT NULL,
    starred boolean DEFAULT false NOT NULL,
    last_viewed_pins_at timestamp(6) without time zone
);


--
-- Name: user_chat_channel_memberships_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_chat_channel_memberships_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_chat_channel_memberships_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_chat_channel_memberships_id_seq OWNED BY public.user_chat_channel_memberships.id;


--
-- Name: user_chat_thread_memberships; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_chat_thread_memberships (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    thread_id bigint NOT NULL,
    last_read_message_id bigint,
    notification_level integer DEFAULT 2 NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    thread_title_prompt_seen boolean DEFAULT false NOT NULL,
    last_unread_message_when_emailed_id bigint
);


--
-- Name: user_chat_thread_memberships_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_chat_thread_memberships_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_chat_thread_memberships_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_chat_thread_memberships_id_seq OWNED BY public.user_chat_thread_memberships.id;


--
-- Name: user_custom_fields; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_custom_fields (
    id integer NOT NULL,
    user_id integer NOT NULL,
    name character varying(256) NOT NULL,
    value text,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: user_custom_fields_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_custom_fields_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_custom_fields_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_custom_fields_id_seq OWNED BY public.user_custom_fields.id;


--
-- Name: user_emails; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_emails (
    id integer NOT NULL,
    user_id integer NOT NULL,
    email character varying(513) NOT NULL,
    "primary" boolean DEFAULT false NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    normalized_email character varying
);


--
-- Name: user_emails_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_emails_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_emails_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_emails_id_seq OWNED BY public.user_emails.id;


--
-- Name: user_exports; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_exports (
    id integer NOT NULL,
    file_name character varying NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    upload_id integer,
    topic_id integer
);


--
-- Name: user_exports_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_exports_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_exports_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_exports_id_seq OWNED BY public.user_exports.id;


--
-- Name: user_field_options; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_field_options (
    id integer NOT NULL,
    user_field_id integer NOT NULL,
    value character varying NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: user_field_options_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_field_options_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_field_options_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_field_options_id_seq OWNED BY public.user_field_options.id;


--
-- Name: user_fields; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_fields (
    id integer NOT NULL,
    name character varying NOT NULL,
    field_type character varying,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    editable boolean DEFAULT false NOT NULL,
    description character varying NOT NULL,
    required boolean DEFAULT true NOT NULL,
    show_on_profile boolean DEFAULT false NOT NULL,
    "position" integer DEFAULT 0,
    show_on_user_card boolean DEFAULT false NOT NULL,
    external_name character varying,
    external_type character varying,
    searchable boolean DEFAULT false NOT NULL,
    requirement integer DEFAULT 0 NOT NULL,
    field_type_enum integer NOT NULL,
    show_on_signup boolean DEFAULT true NOT NULL
);


--
-- Name: user_fields_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_fields_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_fields_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_fields_id_seq OWNED BY public.user_fields.id;


--
-- Name: user_histories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_histories (
    id integer NOT NULL,
    action integer NOT NULL,
    acting_user_id integer,
    target_user_id integer,
    details text,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    context character varying,
    ip_address character varying,
    email character varying,
    subject text,
    previous_value text,
    new_value text,
    topic_id integer,
    admin_only boolean DEFAULT false,
    post_id integer,
    custom_type character varying,
    category_id integer,
    reviewable_id bigint
);


--
-- Name: user_histories_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_histories_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_histories_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_histories_id_seq OWNED BY public.user_histories.id;


--
-- Name: user_ip_address_histories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_ip_address_histories (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    ip_address inet NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: user_ip_address_histories_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_ip_address_histories_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_ip_address_histories_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_ip_address_histories_id_seq OWNED BY public.user_ip_address_histories.id;


--
-- Name: user_notification_schedules; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_notification_schedules (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    day_0_start_time integer NOT NULL,
    day_0_end_time integer NOT NULL,
    day_1_start_time integer NOT NULL,
    day_1_end_time integer NOT NULL,
    day_2_start_time integer NOT NULL,
    day_2_end_time integer NOT NULL,
    day_3_start_time integer NOT NULL,
    day_3_end_time integer NOT NULL,
    day_4_start_time integer NOT NULL,
    day_4_end_time integer NOT NULL,
    day_5_start_time integer NOT NULL,
    day_5_end_time integer NOT NULL,
    day_6_start_time integer NOT NULL,
    day_6_end_time integer NOT NULL
);


--
-- Name: user_notification_schedules_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_notification_schedules_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_notification_schedules_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_notification_schedules_id_seq OWNED BY public.user_notification_schedules.id;


--
-- Name: user_open_ids; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_open_ids (
    id integer NOT NULL,
    user_id integer NOT NULL,
    email character varying NOT NULL,
    url character varying NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    active boolean NOT NULL
);


--
-- Name: user_open_ids_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_open_ids_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_open_ids_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_open_ids_id_seq OWNED BY public.user_open_ids.id;


--
-- Name: user_options; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_options (
    user_id integer NOT NULL,
    mailing_list_mode boolean DEFAULT false NOT NULL,
    email_digests boolean,
    external_links_in_new_tab boolean DEFAULT false NOT NULL,
    enable_quoting boolean DEFAULT true NOT NULL,
    dynamic_favicon boolean DEFAULT false NOT NULL,
    automatically_unpin_topics boolean DEFAULT true NOT NULL,
    digest_after_minutes integer,
    auto_track_topics_after_msecs integer,
    new_topic_duration_minutes integer,
    last_redirected_to_top_at timestamp without time zone,
    email_previous_replies integer DEFAULT 2 NOT NULL,
    email_in_reply_to boolean DEFAULT true NOT NULL,
    like_notification_frequency integer DEFAULT 1 NOT NULL,
    mailing_list_mode_frequency integer DEFAULT 1 NOT NULL,
    include_tl0_in_digests boolean DEFAULT false,
    notification_level_when_replying integer,
    theme_key_seq integer DEFAULT 0 NOT NULL,
    allow_private_messages boolean DEFAULT true NOT NULL,
    homepage_id integer,
    theme_ids integer[] DEFAULT '{}'::integer[] NOT NULL,
    hide_profile_and_presence boolean DEFAULT false NOT NULL,
    text_size_key integer DEFAULT 0 NOT NULL,
    text_size_seq integer DEFAULT 0 NOT NULL,
    email_level integer DEFAULT 1 NOT NULL,
    email_messages_level integer DEFAULT 0 NOT NULL,
    title_count_mode_key integer DEFAULT 0 NOT NULL,
    enable_defer boolean DEFAULT false NOT NULL,
    timezone character varying,
    enable_allowed_pm_users boolean DEFAULT false NOT NULL,
    dark_scheme_id integer,
    skip_new_user_tips boolean DEFAULT false NOT NULL,
    color_scheme_id integer,
    default_calendar integer DEFAULT 0 NOT NULL,
    chat_enabled boolean DEFAULT true NOT NULL,
    only_chat_push_notifications boolean,
    oldest_search_log_date timestamp without time zone,
    chat_sound character varying,
    dismissed_channel_retention_reminder boolean,
    dismissed_dm_retention_reminder boolean,
    bookmark_auto_delete_preference integer DEFAULT 3 NOT NULL,
    ignore_channel_wide_mention boolean,
    chat_email_frequency integer DEFAULT 1 NOT NULL,
    seen_popups integer[],
    policy_email_frequency integer DEFAULT 0 NOT NULL,
    chat_header_indicator_preference integer DEFAULT 0 NOT NULL,
    sidebar_link_to_filtered_list boolean DEFAULT false NOT NULL,
    sidebar_show_count_of_new_items boolean DEFAULT false NOT NULL,
    watched_precedence_over_muted boolean DEFAULT false NOT NULL,
    chat_separate_sidebar_mode integer DEFAULT 0 NOT NULL,
    show_thread_title_prompts boolean DEFAULT true NOT NULL,
    auto_image_caption boolean DEFAULT false NOT NULL,
    enable_smart_lists boolean DEFAULT true NOT NULL,
    hide_profile boolean DEFAULT false NOT NULL,
    hide_presence boolean DEFAULT false NOT NULL,
    chat_send_shortcut integer DEFAULT 0 NOT NULL,
    notification_level_when_assigned integer DEFAULT 3 NOT NULL,
    chat_quick_reaction_type integer DEFAULT 0 NOT NULL,
    chat_quick_reactions_custom character varying,
    ai_search_discoveries boolean DEFAULT true NOT NULL,
    composition_mode integer DEFAULT 1 NOT NULL,
    interface_color_mode integer DEFAULT 1 NOT NULL,
    enable_markdown_monospace_font boolean DEFAULT true NOT NULL,
    notify_on_linked_posts boolean DEFAULT true NOT NULL,
    discourse_rewind_share_publicly boolean DEFAULT false NOT NULL,
    discourse_rewind_dismissed_at timestamp(6) without time zone,
    discourse_rewind_enabled boolean DEFAULT true NOT NULL,
    notify_on_solved boolean DEFAULT true NOT NULL,
    show_original_content boolean DEFAULT false NOT NULL,
    enable_upcoming_change_available_notifications boolean DEFAULT true NOT NULL,
    chat_announce_new_messages boolean DEFAULT true NOT NULL,
    chat_new_message_sound boolean DEFAULT false NOT NULL,
    push_notification_level integer DEFAULT 1 NOT NULL,
    automatically_translate boolean DEFAULT true NOT NULL,
    understood_languages character varying[] DEFAULT '{}'::character varying[] NOT NULL,
    send_shortcut integer DEFAULT 0 NOT NULL,
    ai_ask_ai_default boolean DEFAULT true NOT NULL
);


--
-- Name: user_passwords; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_passwords (
    id integer NOT NULL,
    user_id integer NOT NULL,
    password_hash character varying(64) NOT NULL,
    password_salt character varying(32) NOT NULL,
    password_algorithm character varying(64) NOT NULL,
    password_expired_at timestamp(6) without time zone,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: user_passwords_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_passwords_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_passwords_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_passwords_id_seq OWNED BY public.user_passwords.id;


--
-- Name: user_profile_views; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_profile_views (
    id integer NOT NULL,
    user_profile_id integer NOT NULL,
    viewed_at timestamp without time zone NOT NULL,
    ip_address inet,
    user_id integer
);


--
-- Name: user_profile_views_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_profile_views_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_profile_views_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_profile_views_id_seq OWNED BY public.user_profile_views.id;


--
-- Name: user_profiles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_profiles (
    user_id integer NOT NULL,
    location character varying(3000),
    website character varying(3000),
    bio_raw text,
    bio_cooked text,
    dismissed_banner_key integer,
    bio_cooked_version integer,
    views integer DEFAULT 0 NOT NULL,
    profile_background_upload_id integer,
    card_background_upload_id integer,
    granted_title_badge_id bigint,
    featured_topic_id integer
);


--
-- Name: user_required_fields_versions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_required_fields_versions (
    id bigint NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: user_required_fields_versions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_required_fields_versions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_required_fields_versions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_required_fields_versions_id_seq OWNED BY public.user_required_fields_versions.id;


--
-- Name: user_search_data; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_search_data (
    user_id integer NOT NULL,
    search_data tsvector,
    raw_data text,
    locale text,
    version integer DEFAULT 0
);


--
-- Name: user_second_factors; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_second_factors (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    method integer NOT NULL,
    data character varying NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    last_used timestamp without time zone,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    name character varying(300)
);


--
-- Name: user_second_factors_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_second_factors_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_second_factors_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_second_factors_id_seq OWNED BY public.user_second_factors.id;


--
-- Name: user_security_keys; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_security_keys (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    credential_id character varying NOT NULL,
    public_key character varying NOT NULL,
    factor_type integer DEFAULT 0 NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    name character varying(300) NOT NULL,
    last_used timestamp without time zone,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: user_security_keys_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_security_keys_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_security_keys_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_security_keys_id_seq OWNED BY public.user_security_keys.id;


--
-- Name: user_stats; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_stats (
    user_id integer NOT NULL,
    topics_entered integer DEFAULT 0 NOT NULL,
    time_read integer DEFAULT 0 NOT NULL,
    days_visited integer DEFAULT 0 NOT NULL,
    posts_read_count integer DEFAULT 0 NOT NULL,
    likes_given integer DEFAULT 0 NOT NULL,
    likes_received integer DEFAULT 0 NOT NULL,
    new_since timestamp without time zone NOT NULL,
    read_faq timestamp without time zone,
    first_post_created_at timestamp without time zone,
    post_count integer DEFAULT 0 NOT NULL,
    topic_count integer DEFAULT 0 NOT NULL,
    bounce_score double precision DEFAULT 0 NOT NULL,
    reset_bounce_score_after timestamp without time zone,
    flags_agreed integer DEFAULT 0 NOT NULL,
    flags_disagreed integer DEFAULT 0 NOT NULL,
    flags_ignored integer DEFAULT 0 NOT NULL,
    first_unread_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    distinct_badge_count integer DEFAULT 0 NOT NULL,
    first_unread_pm_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    digest_attempted_at timestamp without time zone,
    post_edits_count integer,
    draft_count integer DEFAULT 0 NOT NULL,
    pending_posts_count integer DEFAULT 0 NOT NULL
);


--
-- Name: user_statuses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_statuses (
    user_id integer NOT NULL,
    emoji character varying NOT NULL,
    description character varying NOT NULL,
    set_at timestamp(6) without time zone NOT NULL,
    ends_at timestamp(6) without time zone
);


--
-- Name: user_statuses_user_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_statuses_user_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_statuses_user_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_statuses_user_id_seq OWNED BY public.user_statuses.user_id;


--
-- Name: user_uploads; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_uploads (
    id bigint NOT NULL,
    upload_id integer NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL
);


--
-- Name: user_uploads_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_uploads_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_uploads_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_uploads_id_seq OWNED BY public.user_uploads.id;


--
-- Name: user_visit_daily_rollups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_visit_daily_rollups (
    id bigint NOT NULL,
    date date NOT NULL,
    dau bigint NOT NULL,
    mau bigint NOT NULL
);


--
-- Name: user_visit_daily_rollups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_visit_daily_rollups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_visit_daily_rollups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_visit_daily_rollups_id_seq OWNED BY public.user_visit_daily_rollups.id;


--
-- Name: user_visits; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_visits (
    id integer NOT NULL,
    user_id integer NOT NULL,
    visited_at date NOT NULL,
    posts_read integer DEFAULT 0,
    mobile boolean DEFAULT false,
    time_read integer DEFAULT 0 NOT NULL
);


--
-- Name: user_visits_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_visits_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_visits_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_visits_id_seq OWNED BY public.user_visits.id;


--
-- Name: user_warnings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_warnings (
    id integer NOT NULL,
    topic_id integer NOT NULL,
    user_id integer NOT NULL,
    created_by_id integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: user_warnings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_warnings_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_warnings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_warnings_id_seq OWNED BY public.user_warnings.id;


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id integer NOT NULL,
    username character varying(60) NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    name character varying,
    last_posted_at timestamp without time zone,
    active boolean DEFAULT false NOT NULL,
    username_lower character varying(60) NOT NULL,
    last_seen_at timestamp without time zone,
    admin boolean DEFAULT false NOT NULL,
    last_emailed_at timestamp without time zone,
    trust_level integer NOT NULL,
    approved boolean DEFAULT false NOT NULL,
    approved_by_id integer,
    approved_at timestamp without time zone,
    previous_visit_at timestamp without time zone,
    suspended_at timestamp without time zone,
    suspended_till timestamp without time zone,
    date_of_birth date,
    views integer DEFAULT 0 NOT NULL,
    flag_level integer DEFAULT 0 NOT NULL,
    ip_address inet,
    moderator boolean DEFAULT false,
    title character varying,
    uploaded_avatar_id integer,
    locale character varying(10),
    primary_group_id integer,
    registration_ip_address inet,
    staged boolean DEFAULT false NOT NULL,
    first_seen_at timestamp without time zone,
    silenced_till timestamp without time zone,
    group_locked_trust_level integer,
    manual_locked_trust_level integer,
    secure_identifier character varying,
    flair_group_id integer,
    last_seen_reviewable_id integer,
    required_fields_version integer,
    seen_notification_id bigint DEFAULT 0 NOT NULL
);


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.users_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.users_id_seq OWNED BY public.users.id;


--
-- Name: voice_co_presences; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.voice_co_presences (
    id bigint NOT NULL,
    user_id_1 integer NOT NULL,
    user_id_2 integer NOT NULL,
    date date NOT NULL,
    total_seconds integer DEFAULT 0 NOT NULL,
    session_count integer DEFAULT 0 NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    CONSTRAINT chk_voice_co_presences_user_order CHECK ((user_id_1 < user_id_2))
);


--
-- Name: voice_co_presences_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.voice_co_presences_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: voice_co_presences_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.voice_co_presences_id_seq OWNED BY public.voice_co_presences.id;


--
-- Name: voice_invites; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.voice_invites (
    id bigint NOT NULL,
    room_id bigint NOT NULL,
    user_id bigint NOT NULL,
    invited_by_id bigint NOT NULL,
    source integer DEFAULT 0 NOT NULL,
    redeemed_at timestamp(6) without time zone,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: voice_invites_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.voice_invites_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: voice_invites_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.voice_invites_id_seq OWNED BY public.voice_invites.id;


--
-- Name: voice_recordings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.voice_recordings (
    id bigint NOT NULL,
    room_id bigint NOT NULL,
    started_by_id bigint NOT NULL,
    egress_id character varying NOT NULL,
    status integer DEFAULT 0 NOT NULL,
    filepath character varying NOT NULL,
    filename character varying,
    location character varying,
    duration_ms bigint,
    size_bytes bigint,
    started_at timestamp(6) without time zone NOT NULL,
    ended_at timestamp(6) without time zone,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: voice_recordings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.voice_recordings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: voice_recordings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.voice_recordings_id_seq OWNED BY public.voice_recordings.id;


--
-- Name: voice_room_memberships; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.voice_room_memberships (
    id bigint NOT NULL,
    room_id bigint NOT NULL,
    user_id bigint NOT NULL,
    role integer DEFAULT 0 NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: voice_room_memberships_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.voice_room_memberships_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: voice_room_memberships_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.voice_room_memberships_id_seq OWNED BY public.voice_room_memberships.id;


--
-- Name: voice_rooms; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.voice_rooms (
    id bigint NOT NULL,
    name character varying NOT NULL,
    slug character varying NOT NULL,
    description text,
    public boolean DEFAULT false NOT NULL,
    max_participants integer,
    creator_id bigint NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL,
    cooked_description text,
    room_type integer DEFAULT 0 NOT NULL,
    video_enabled boolean DEFAULT true NOT NULL,
    chat_channel_id bigint,
    chat_idle_minutes integer DEFAULT 15 NOT NULL,
    livekit_enabled boolean DEFAULT false NOT NULL,
    max_quality_profile integer,
    ephemeral boolean DEFAULT false NOT NULL,
    last_occupied_at timestamp(6) without time zone
);


--
-- Name: voice_rooms_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.voice_rooms_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: voice_rooms_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.voice_rooms_id_seq OWNED BY public.voice_rooms.id;


--
-- Name: voice_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.voice_sessions (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    room_id bigint NOT NULL,
    joined_at timestamp(6) without time zone NOT NULL,
    left_at timestamp(6) without time zone,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: voice_sessions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.voice_sessions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: voice_sessions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.voice_sessions_id_seq OWNED BY public.voice_sessions.id;


--
-- Name: watched_word_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.watched_word_groups (
    id bigint NOT NULL,
    action integer NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: watched_word_groups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.watched_word_groups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: watched_word_groups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.watched_word_groups_id_seq OWNED BY public.watched_word_groups.id;


--
-- Name: watched_words; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.watched_words (
    id integer NOT NULL,
    word character varying NOT NULL,
    action integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    replacement character varying,
    case_sensitive boolean DEFAULT false NOT NULL,
    watched_word_group_id bigint,
    html boolean DEFAULT false NOT NULL
);


--
-- Name: watched_words_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.watched_words_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: watched_words_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.watched_words_id_seq OWNED BY public.watched_words.id;


--
-- Name: web_crawler_requests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.web_crawler_requests (
    id bigint NOT NULL,
    date date NOT NULL,
    user_agent character varying NOT NULL,
    count integer DEFAULT 0 NOT NULL
);


--
-- Name: web_crawler_requests_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.web_crawler_requests_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: web_crawler_requests_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.web_crawler_requests_id_seq OWNED BY public.web_crawler_requests.id;


--
-- Name: web_hook_event_types; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.web_hook_event_types (
    id integer NOT NULL,
    name character varying NOT NULL,
    "group" integer
);


--
-- Name: web_hook_event_types_hooks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.web_hook_event_types_hooks (
    web_hook_id integer NOT NULL,
    web_hook_event_type_id integer NOT NULL
);


--
-- Name: web_hook_event_types_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.web_hook_event_types_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: web_hook_event_types_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.web_hook_event_types_id_seq OWNED BY public.web_hook_event_types.id;


--
-- Name: web_hook_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.web_hook_events (
    id bigint NOT NULL,
    web_hook_id integer NOT NULL,
    headers character varying,
    payload text,
    status integer DEFAULT 0,
    response_headers character varying,
    response_body text,
    duration integer DEFAULT 0,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: web_hook_events_daily_aggregates; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.web_hook_events_daily_aggregates (
    id bigint NOT NULL,
    web_hook_id bigint NOT NULL,
    date date,
    successful_event_count integer,
    failed_event_count integer,
    mean_duration integer DEFAULT 0,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);


--
-- Name: web_hook_events_daily_aggregates_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.web_hook_events_daily_aggregates_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: web_hook_events_daily_aggregates_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.web_hook_events_daily_aggregates_id_seq OWNED BY public.web_hook_events_daily_aggregates.id;


--
-- Name: web_hook_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.web_hook_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: web_hook_events_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.web_hook_events_id_seq OWNED BY public.web_hook_events.id;


--
-- Name: web_hooks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.web_hooks (
    id integer NOT NULL,
    payload_url character varying NOT NULL,
    content_type integer DEFAULT 1 NOT NULL,
    last_delivery_status integer DEFAULT 1 NOT NULL,
    status integer DEFAULT 1 NOT NULL,
    secret character varying DEFAULT ''::character varying,
    wildcard_web_hook boolean DEFAULT false NOT NULL,
    verify_certificate boolean DEFAULT true NOT NULL,
    active boolean DEFAULT false NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


--
-- Name: web_hooks_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.web_hooks_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: web_hooks_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.web_hooks_id_seq OWNED BY public.web_hooks.id;


--
-- Name: access_control_lists id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.access_control_lists ALTER COLUMN id SET DEFAULT nextval('public.access_control_lists_id_seq'::regclass);


--
-- Name: ad_plugin_house_ads id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ad_plugin_house_ads ALTER COLUMN id SET DEFAULT nextval('public.ad_plugin_house_ads_id_seq'::regclass);


--
-- Name: ad_plugin_impressions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ad_plugin_impressions ALTER COLUMN id SET DEFAULT nextval('public.ad_plugin_impressions_id_seq'::regclass);


--
-- Name: admin_dashboard_reports id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.admin_dashboard_reports ALTER COLUMN id SET DEFAULT nextval('public.admin_dashboard_reports_id_seq'::regclass);


--
-- Name: admin_dashboard_sections id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.admin_dashboard_sections ALTER COLUMN id SET DEFAULT nextval('public.admin_dashboard_sections_id_seq'::regclass);


--
-- Name: admin_notices id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.admin_notices ALTER COLUMN id SET DEFAULT nextval('public.admin_notices_id_seq'::regclass);


--
-- Name: ai_agent_mcp_servers id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_agent_mcp_servers ALTER COLUMN id SET DEFAULT nextval('public.ai_agent_mcp_servers_id_seq'::regclass);


--
-- Name: ai_agents id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_agents ALTER COLUMN id SET DEFAULT nextval('public.ai_agents_id_seq'::regclass);


--
-- Name: ai_api_audit_logs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_api_audit_logs ALTER COLUMN id SET DEFAULT nextval('public.ai_api_audit_logs_id_seq'::regclass);


--
-- Name: ai_api_request_stats id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_api_request_stats ALTER COLUMN id SET DEFAULT nextval('public.ai_api_request_stats_id_seq'::regclass);


--
-- Name: ai_artifact_key_values id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_artifact_key_values ALTER COLUMN id SET DEFAULT nextval('public.ai_artifact_key_values_id_seq'::regclass);


--
-- Name: ai_artifact_versions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_artifact_versions ALTER COLUMN id SET DEFAULT nextval('public.ai_artifact_versions_id_seq'::regclass);


--
-- Name: ai_artifacts id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_artifacts ALTER COLUMN id SET DEFAULT nextval('public.ai_artifacts_id_seq'::regclass);


--
-- Name: ai_mcp_oauth_tokens id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_mcp_oauth_tokens ALTER COLUMN id SET DEFAULT nextval('public.ai_mcp_oauth_tokens_id_seq'::regclass);


--
-- Name: ai_mcp_servers id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_mcp_servers ALTER COLUMN id SET DEFAULT nextval('public.ai_mcp_servers_id_seq'::regclass);


--
-- Name: ai_moderation_settings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_moderation_settings ALTER COLUMN id SET DEFAULT nextval('public.ai_moderation_settings_id_seq'::regclass);


--
-- Name: ai_post_image_captions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_post_image_captions ALTER COLUMN id SET DEFAULT nextval('public.ai_post_image_captions_id_seq'::regclass);


--
-- Name: ai_secrets id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_secrets ALTER COLUMN id SET DEFAULT nextval('public.ai_secrets_id_seq'::regclass);


--
-- Name: ai_spam_logs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_spam_logs ALTER COLUMN id SET DEFAULT nextval('public.ai_spam_logs_id_seq'::regclass);


--
-- Name: ai_summaries id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_summaries ALTER COLUMN id SET DEFAULT nextval('public.ai_summaries_id_seq'::regclass);


--
-- Name: ai_tool_actions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_tool_actions ALTER COLUMN id SET DEFAULT nextval('public.ai_tool_actions_id_seq'::regclass);


--
-- Name: ai_tool_secret_bindings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_tool_secret_bindings ALTER COLUMN id SET DEFAULT nextval('public.ai_tool_secret_bindings_id_seq'::regclass);


--
-- Name: ai_tools id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_tools ALTER COLUMN id SET DEFAULT nextval('public.ai_tools_id_seq'::regclass);


--
-- Name: allowed_pm_users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.allowed_pm_users ALTER COLUMN id SET DEFAULT nextval('public.allowed_pm_users_id_seq'::regclass);


--
-- Name: anonymous_users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.anonymous_users ALTER COLUMN id SET DEFAULT nextval('public.anonymous_users_id_seq'::regclass);


--
-- Name: api_key_scopes id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.api_key_scopes ALTER COLUMN id SET DEFAULT nextval('public.api_key_scopes_id_seq'::regclass);


--
-- Name: api_keys id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.api_keys ALTER COLUMN id SET DEFAULT nextval('public.api_keys_id_seq'::regclass);


--
-- Name: application_requests id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.application_requests ALTER COLUMN id SET DEFAULT nextval('public.application_requests_id_seq'::regclass);


--
-- Name: ask_ai_logs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ask_ai_logs ALTER COLUMN id SET DEFAULT nextval('public.ask_ai_logs_id_seq'::regclass);


--
-- Name: assignments id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.assignments ALTER COLUMN id SET DEFAULT nextval('public.assignments_id_seq'::regclass);


--
-- Name: associated_groups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.associated_groups ALTER COLUMN id SET DEFAULT nextval('public.associated_groups_id_seq'::regclass);


--
-- Name: backup_draft_posts id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.backup_draft_posts ALTER COLUMN id SET DEFAULT nextval('public.backup_draft_posts_id_seq'::regclass);


--
-- Name: backup_draft_topics id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.backup_draft_topics ALTER COLUMN id SET DEFAULT nextval('public.backup_draft_topics_id_seq'::regclass);


--
-- Name: backup_metadata id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.backup_metadata ALTER COLUMN id SET DEFAULT nextval('public.backup_metadata_id_seq'::regclass);


--
-- Name: badge_groupings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.badge_groupings ALTER COLUMN id SET DEFAULT nextval('public.badge_groupings_id_seq'::regclass);


--
-- Name: badge_types id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.badge_types ALTER COLUMN id SET DEFAULT nextval('public.badge_types_id_seq'::regclass);


--
-- Name: badges id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.badges ALTER COLUMN id SET DEFAULT nextval('public.badges_id_seq'::regclass);


--
-- Name: bookmarks id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bookmarks ALTER COLUMN id SET DEFAULT nextval('public.bookmarks_id_seq'::regclass);


--
-- Name: browser_pageview_country_daily_rollups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.browser_pageview_country_daily_rollups ALTER COLUMN id SET DEFAULT nextval('public.browser_pageview_country_daily_rollups_id_seq'::regclass);


--
-- Name: browser_pageview_crawler_daily_rollups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.browser_pageview_crawler_daily_rollups ALTER COLUMN id SET DEFAULT nextval('public.browser_pageview_crawler_daily_rollups_id_seq'::regclass);


--
-- Name: browser_pageview_entry_url_daily_rollups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.browser_pageview_entry_url_daily_rollups ALTER COLUMN id SET DEFAULT nextval('public.browser_pageview_entry_url_daily_rollups_id_seq'::regclass);


--
-- Name: browser_pageview_event_scores id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.browser_pageview_event_scores ALTER COLUMN id SET DEFAULT nextval('public.browser_pageview_event_scores_id_seq'::regclass);


--
-- Name: browser_pageview_events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.browser_pageview_events ALTER COLUMN id SET DEFAULT nextval('public.browser_pageview_events_id_seq'::regclass);


--
-- Name: browser_pageview_referrer_daily_rollups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.browser_pageview_referrer_daily_rollups ALTER COLUMN id SET DEFAULT nextval('public.browser_pageview_referrer_daily_rollups_id_seq'::regclass);


--
-- Name: browser_pageview_session_engagement_daily_rollups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.browser_pageview_session_engagement_daily_rollups ALTER COLUMN id SET DEFAULT nextval('public.browser_pageview_session_engagement_daily_rollups_id_seq'::regclass);


--
-- Name: browser_pageview_session_engagements id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.browser_pageview_session_engagements ALTER COLUMN id SET DEFAULT nextval('public.browser_pageview_session_engagements_id_seq'::regclass);


--
-- Name: calendar_events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.calendar_events ALTER COLUMN id SET DEFAULT nextval('public.calendar_events_id_seq'::regclass);


--
-- Name: categories id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.categories ALTER COLUMN id SET DEFAULT nextval('public.categories_id_seq'::regclass);


--
-- Name: category_activity_daily_rollups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_activity_daily_rollups ALTER COLUMN id SET DEFAULT nextval('public.category_activity_daily_rollups_id_seq'::regclass);


--
-- Name: category_custom_fields id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_custom_fields ALTER COLUMN id SET DEFAULT nextval('public.category_custom_fields_id_seq'::regclass);


--
-- Name: category_featured_topics id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_featured_topics ALTER COLUMN id SET DEFAULT nextval('public.category_featured_topics_id_seq'::regclass);


--
-- Name: category_form_templates id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_form_templates ALTER COLUMN id SET DEFAULT nextval('public.category_form_templates_id_seq'::regclass);


--
-- Name: category_groups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_groups ALTER COLUMN id SET DEFAULT nextval('public.category_groups_id_seq'::regclass);


--
-- Name: category_localizations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_localizations ALTER COLUMN id SET DEFAULT nextval('public.category_localizations_id_seq'::regclass);


--
-- Name: category_moderation_groups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_moderation_groups ALTER COLUMN id SET DEFAULT nextval('public.category_moderation_groups_id_seq'::regclass);


--
-- Name: category_posting_review_groups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_posting_review_groups ALTER COLUMN id SET DEFAULT nextval('public.category_posting_review_groups_id_seq'::regclass);


--
-- Name: category_required_tag_groups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_required_tag_groups ALTER COLUMN id SET DEFAULT nextval('public.category_required_tag_groups_id_seq'::regclass);


--
-- Name: category_settings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_settings ALTER COLUMN id SET DEFAULT nextval('public.category_settings_id_seq'::regclass);


--
-- Name: category_tag_groups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_tag_groups ALTER COLUMN id SET DEFAULT nextval('public.category_tag_groups_id_seq'::regclass);


--
-- Name: category_tag_stats id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_tag_stats ALTER COLUMN id SET DEFAULT nextval('public.category_tag_stats_id_seq'::regclass);


--
-- Name: category_tags id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_tags ALTER COLUMN id SET DEFAULT nextval('public.category_tags_id_seq'::regclass);


--
-- Name: category_users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_users ALTER COLUMN id SET DEFAULT nextval('public.category_users_id_seq'::regclass);


--
-- Name: chat_channel_archives id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_channel_archives ALTER COLUMN id SET DEFAULT nextval('public.chat_channel_archives_id_seq'::regclass);


--
-- Name: chat_channel_custom_fields id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_channel_custom_fields ALTER COLUMN id SET DEFAULT nextval('public.chat_channel_custom_fields_id_seq'::regclass);


--
-- Name: chat_channels id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_channels ALTER COLUMN id SET DEFAULT nextval('public.chat_channels_id_seq'::regclass);


--
-- Name: chat_drafts id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_drafts ALTER COLUMN id SET DEFAULT nextval('public.chat_drafts_id_seq'::regclass);


--
-- Name: chat_mentions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_mentions ALTER COLUMN id SET DEFAULT nextval('public.chat_mentions_id_seq'::regclass);


--
-- Name: chat_message_custom_fields id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_custom_fields ALTER COLUMN id SET DEFAULT nextval('public.chat_message_custom_fields_id_seq'::regclass);


--
-- Name: chat_message_custom_prompts id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_custom_prompts ALTER COLUMN id SET DEFAULT nextval('public.chat_message_custom_prompts_id_seq'::regclass);


--
-- Name: chat_message_hotlinked_media id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_hotlinked_media ALTER COLUMN id SET DEFAULT nextval('public.chat_message_hotlinked_media_id_seq'::regclass);


--
-- Name: chat_message_interactions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_interactions ALTER COLUMN id SET DEFAULT nextval('public.chat_message_interactions_id_seq'::regclass);


--
-- Name: chat_message_links id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_links ALTER COLUMN id SET DEFAULT nextval('public.chat_message_links_id_seq'::regclass);


--
-- Name: chat_message_reactions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_reactions ALTER COLUMN id SET DEFAULT nextval('public.chat_message_reactions_id_seq'::regclass);


--
-- Name: chat_message_revisions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_revisions ALTER COLUMN id SET DEFAULT nextval('public.chat_message_revisions_id_seq'::regclass);


--
-- Name: chat_message_search_data chat_message_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_search_data ALTER COLUMN chat_message_id SET DEFAULT nextval('public.chat_message_search_data_chat_message_id_seq'::regclass);


--
-- Name: chat_messages id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_messages ALTER COLUMN id SET DEFAULT nextval('public.chat_messages_id_seq'::regclass);


--
-- Name: chat_pinned_messages id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_pinned_messages ALTER COLUMN id SET DEFAULT nextval('public.chat_pinned_messages_id_seq'::regclass);


--
-- Name: chat_thread_custom_fields id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_thread_custom_fields ALTER COLUMN id SET DEFAULT nextval('public.chat_thread_custom_fields_id_seq'::regclass);


--
-- Name: chat_threads id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_threads ALTER COLUMN id SET DEFAULT nextval('public.chat_threads_id_seq'::regclass);


--
-- Name: chat_webhook_events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_webhook_events ALTER COLUMN id SET DEFAULT nextval('public.chat_webhook_events_id_seq'::regclass);


--
-- Name: child_themes id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.child_themes ALTER COLUMN id SET DEFAULT nextval('public.child_themes_id_seq'::regclass);


--
-- Name: classification_results id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.classification_results ALTER COLUMN id SET DEFAULT nextval('public.classification_results_id_seq'::regclass);


--
-- Name: color_scheme_colors id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.color_scheme_colors ALTER COLUMN id SET DEFAULT nextval('public.color_scheme_colors_id_seq'::regclass);


--
-- Name: color_schemes id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.color_schemes ALTER COLUMN id SET DEFAULT nextval('public.color_schemes_id_seq'::regclass);


--
-- Name: completion_prompts id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.completion_prompts ALTER COLUMN id SET DEFAULT nextval('public.completion_prompts_id_seq'::regclass);


--
-- Name: custom_emojis id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.custom_emojis ALTER COLUMN id SET DEFAULT nextval('public.custom_emojis_id_seq'::regclass);


--
-- Name: data_explorer_queries id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.data_explorer_queries ALTER COLUMN id SET DEFAULT nextval('public.data_explorer_queries_id_seq'::regclass);


--
-- Name: data_explorer_query_groups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.data_explorer_query_groups ALTER COLUMN id SET DEFAULT nextval('public.data_explorer_query_groups_id_seq'::regclass);


--
-- Name: data_explorer_query_stats id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.data_explorer_query_stats ALTER COLUMN id SET DEFAULT nextval('public.data_explorer_query_stats_id_seq'::regclass);


--
-- Name: developers id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.developers ALTER COLUMN id SET DEFAULT nextval('public.developers_id_seq'::regclass);


--
-- Name: direct_message_channels id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.direct_message_channels ALTER COLUMN id SET DEFAULT nextval('public.direct_message_channels_id_seq'::regclass);


--
-- Name: direct_message_users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.direct_message_users ALTER COLUMN id SET DEFAULT nextval('public.direct_message_users_id_seq'::regclass);


--
-- Name: directory_columns id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.directory_columns ALTER COLUMN id SET DEFAULT nextval('public.directory_columns_id_seq'::regclass);


--
-- Name: directory_items id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.directory_items ALTER COLUMN id SET DEFAULT nextval('public.directory_items_id_seq'::regclass);


--
-- Name: discourse_ai_ai_bot_conversation_stars id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_ai_ai_bot_conversation_stars ALTER COLUMN id SET DEFAULT nextval('public.discourse_ai_ai_bot_conversation_stars_id_seq'::regclass);


--
-- Name: discourse_automation_automations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_automation_automations ALTER COLUMN id SET DEFAULT nextval('public.discourse_automation_automations_id_seq'::regclass);


--
-- Name: discourse_automation_fields id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_automation_fields ALTER COLUMN id SET DEFAULT nextval('public.discourse_automation_fields_id_seq'::regclass);


--
-- Name: discourse_automation_pending_automations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_automation_pending_automations ALTER COLUMN id SET DEFAULT nextval('public.discourse_automation_pending_automations_id_seq'::regclass);


--
-- Name: discourse_automation_pending_pms id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_automation_pending_pms ALTER COLUMN id SET DEFAULT nextval('public.discourse_automation_pending_pms_id_seq'::regclass);


--
-- Name: discourse_automation_stats id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_automation_stats ALTER COLUMN id SET DEFAULT nextval('public.discourse_automation_stats_id_seq'::regclass);


--
-- Name: discourse_automation_user_global_notices id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_automation_user_global_notices ALTER COLUMN id SET DEFAULT nextval('public.discourse_automation_user_global_notices_id_seq'::regclass);


--
-- Name: discourse_calendar_disabled_holidays id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_calendar_disabled_holidays ALTER COLUMN id SET DEFAULT nextval('public.discourse_calendar_disabled_holidays_id_seq'::regclass);


--
-- Name: discourse_calendar_post_event_dates id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_calendar_post_event_dates ALTER COLUMN id SET DEFAULT nextval('public.discourse_calendar_post_event_dates_id_seq'::regclass);


--
-- Name: discourse_kanban_board_histories id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_kanban_board_histories ALTER COLUMN id SET DEFAULT nextval('public.discourse_kanban_board_histories_id_seq'::regclass);


--
-- Name: discourse_kanban_boards id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_kanban_boards ALTER COLUMN id SET DEFAULT nextval('public.discourse_kanban_boards_id_seq'::regclass);


--
-- Name: discourse_kanban_card_histories id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_kanban_card_histories ALTER COLUMN id SET DEFAULT nextval('public.discourse_kanban_card_histories_id_seq'::regclass);


--
-- Name: discourse_kanban_cards id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_kanban_cards ALTER COLUMN id SET DEFAULT nextval('public.discourse_kanban_cards_id_seq'::regclass);


--
-- Name: discourse_kanban_columns id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_kanban_columns ALTER COLUMN id SET DEFAULT nextval('public.discourse_kanban_columns_id_seq'::regclass);


--
-- Name: discourse_post_event_events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_post_event_events ALTER COLUMN id SET DEFAULT nextval('public.discourse_post_event_events_id_seq'::regclass);


--
-- Name: discourse_post_event_hosts id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_post_event_hosts ALTER COLUMN id SET DEFAULT nextval('public.discourse_post_event_hosts_id_seq'::regclass);


--
-- Name: discourse_post_event_invitees id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_post_event_invitees ALTER COLUMN id SET DEFAULT nextval('public.discourse_post_event_invitees_id_seq'::regclass);


--
-- Name: discourse_reactions_reaction_users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_reactions_reaction_users ALTER COLUMN id SET DEFAULT nextval('public.discourse_reactions_reaction_users_id_seq'::regclass);


--
-- Name: discourse_reactions_reactions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_reactions_reactions ALTER COLUMN id SET DEFAULT nextval('public.discourse_reactions_reactions_id_seq'::regclass);


--
-- Name: discourse_rss_polling_poll_attempts id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_rss_polling_poll_attempts ALTER COLUMN id SET DEFAULT nextval('public.discourse_rss_polling_poll_attempts_id_seq'::regclass);


--
-- Name: discourse_rss_polling_rss_feeds id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_rss_polling_rss_feeds ALTER COLUMN id SET DEFAULT nextval('public.discourse_rss_polling_rss_feeds_id_seq'::regclass);


--
-- Name: discourse_solved_shared_issues id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_solved_shared_issues ALTER COLUMN id SET DEFAULT nextval('public.discourse_solved_shared_issues_id_seq'::regclass);


--
-- Name: discourse_solved_solved_topics id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_solved_solved_topics ALTER COLUMN id SET DEFAULT nextval('public.discourse_solved_solved_topics_id_seq'::regclass);


--
-- Name: discourse_solved_topic_answers id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_solved_topic_answers ALTER COLUMN id SET DEFAULT nextval('public.discourse_solved_topic_answers_id_seq'::regclass);


--
-- Name: discourse_subscriptions_customers id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_subscriptions_customers ALTER COLUMN id SET DEFAULT nextval('public.discourse_subscriptions_customers_id_seq'::regclass);


--
-- Name: discourse_subscriptions_products id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_subscriptions_products ALTER COLUMN id SET DEFAULT nextval('public.discourse_subscriptions_products_id_seq'::regclass);


--
-- Name: discourse_subscriptions_subscriptions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_subscriptions_subscriptions ALTER COLUMN id SET DEFAULT nextval('public.discourse_subscriptions_subscriptions_id_seq'::regclass);


--
-- Name: discourse_templates_usage_count id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_templates_usage_count ALTER COLUMN id SET DEFAULT nextval('public.discourse_templates_usage_count_id_seq'::regclass);


--
-- Name: discourse_workflows_ai_authoring_sessions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_ai_authoring_sessions ALTER COLUMN id SET DEFAULT nextval('public.discourse_workflows_ai_authoring_sessions_id_seq'::regclass);


--
-- Name: discourse_workflows_credentials id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_credentials ALTER COLUMN id SET DEFAULT nextval('public.discourse_workflows_credentials_id_seq'::regclass);


--
-- Name: discourse_workflows_data_tables id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_data_tables ALTER COLUMN id SET DEFAULT nextval('public.discourse_workflows_data_tables_id_seq'::regclass);


--
-- Name: discourse_workflows_execution_stats id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_execution_stats ALTER COLUMN id SET DEFAULT nextval('public.discourse_workflows_execution_stats_id_seq'::regclass);


--
-- Name: discourse_workflows_executions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_executions ALTER COLUMN id SET DEFAULT nextval('public.discourse_workflows_executions_id_seq'::regclass);


--
-- Name: discourse_workflows_tags id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_tags ALTER COLUMN id SET DEFAULT nextval('public.discourse_workflows_tags_id_seq'::regclass);


--
-- Name: discourse_workflows_variables id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_variables ALTER COLUMN id SET DEFAULT nextval('public.discourse_workflows_variables_id_seq'::regclass);


--
-- Name: discourse_workflows_webhooks id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_webhooks ALTER COLUMN id SET DEFAULT nextval('public.discourse_workflows_webhooks_id_seq'::regclass);


--
-- Name: discourse_workflows_workflow_call_runs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_workflow_call_runs ALTER COLUMN id SET DEFAULT nextval('public.discourse_workflows_workflow_call_runs_id_seq'::regclass);


--
-- Name: discourse_workflows_workflow_dependencies id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_workflow_dependencies ALTER COLUMN id SET DEFAULT nextval('public.discourse_workflows_workflow_dependencies_id_seq'::regclass);


--
-- Name: discourse_workflows_workflow_publish_history id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_workflow_publish_history ALTER COLUMN id SET DEFAULT nextval('public.discourse_workflows_workflow_publish_history_id_seq'::regclass);


--
-- Name: discourse_workflows_workflow_tags id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_workflow_tags ALTER COLUMN id SET DEFAULT nextval('public.discourse_workflows_workflow_tags_id_seq'::regclass);


--
-- Name: discourse_workflows_workflows id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_workflows ALTER COLUMN id SET DEFAULT nextval('public.discourse_workflows_workflows_id_seq'::regclass);


--
-- Name: dismissed_topic_users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dismissed_topic_users ALTER COLUMN id SET DEFAULT nextval('public.dismissed_topic_users_id_seq'::regclass);


--
-- Name: do_not_disturb_timings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.do_not_disturb_timings ALTER COLUMN id SET DEFAULT nextval('public.do_not_disturb_timings_id_seq'::regclass);


--
-- Name: draft_sequences id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.draft_sequences ALTER COLUMN id SET DEFAULT nextval('public.draft_sequences_id_seq'::regclass);


--
-- Name: drafts id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drafts ALTER COLUMN id SET DEFAULT nextval('public.drafts_id_seq'::regclass);


--
-- Name: email_change_requests id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.email_change_requests ALTER COLUMN id SET DEFAULT nextval('public.email_change_requests_id_seq'::regclass);


--
-- Name: email_login_codes id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.email_login_codes ALTER COLUMN id SET DEFAULT nextval('public.email_login_codes_id_seq'::regclass);


--
-- Name: email_logs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.email_logs ALTER COLUMN id SET DEFAULT nextval('public.email_logs_id_seq'::regclass);


--
-- Name: email_tokens id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.email_tokens ALTER COLUMN id SET DEFAULT nextval('public.email_tokens_id_seq'::regclass);


--
-- Name: embeddable_host_tags id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.embeddable_host_tags ALTER COLUMN id SET DEFAULT nextval('public.embeddable_host_tags_id_seq'::regclass);


--
-- Name: embeddable_hosts id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.embeddable_hosts ALTER COLUMN id SET DEFAULT nextval('public.embeddable_hosts_id_seq'::regclass);


--
-- Name: embedding_definitions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.embedding_definitions ALTER COLUMN id SET DEFAULT nextval('public.embedding_definitions_id_seq'::regclass);


--
-- Name: external_upload_stubs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.external_upload_stubs ALTER COLUMN id SET DEFAULT nextval('public.external_upload_stubs_id_seq'::regclass);


--
-- Name: flags id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.flags ALTER COLUMN id SET DEFAULT nextval('public.flags_id_seq'::regclass);


--
-- Name: form_templates id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.form_templates ALTER COLUMN id SET DEFAULT nextval('public.form_templates_id_seq'::regclass);


--
-- Name: gamification_leaderboard_scores id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gamification_leaderboard_scores ALTER COLUMN id SET DEFAULT nextval('public.gamification_leaderboard_scores_id_seq'::regclass);


--
-- Name: gamification_leaderboards id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gamification_leaderboards ALTER COLUMN id SET DEFAULT nextval('public.gamification_leaderboards_id_seq'::regclass);


--
-- Name: gamification_score_events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gamification_score_events ALTER COLUMN id SET DEFAULT nextval('public.gamification_score_events_id_seq'::regclass);


--
-- Name: gamification_scores id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gamification_scores ALTER COLUMN id SET DEFAULT nextval('public.gamification_scores_id_seq'::regclass);


--
-- Name: github_commits id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.github_commits ALTER COLUMN id SET DEFAULT nextval('public.github_commits_id_seq'::regclass);


--
-- Name: github_repos id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.github_repos ALTER COLUMN id SET DEFAULT nextval('public.github_repos_id_seq'::regclass);


--
-- Name: group_archived_messages id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.group_archived_messages ALTER COLUMN id SET DEFAULT nextval('public.group_archived_messages_id_seq'::regclass);


--
-- Name: group_associated_groups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.group_associated_groups ALTER COLUMN id SET DEFAULT nextval('public.group_associated_groups_id_seq'::regclass);


--
-- Name: group_category_notification_defaults id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.group_category_notification_defaults ALTER COLUMN id SET DEFAULT nextval('public.group_category_notification_defaults_id_seq'::regclass);


--
-- Name: group_custom_fields id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.group_custom_fields ALTER COLUMN id SET DEFAULT nextval('public.group_custom_fields_id_seq'::regclass);


--
-- Name: group_histories id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.group_histories ALTER COLUMN id SET DEFAULT nextval('public.group_histories_id_seq'::regclass);


--
-- Name: group_mentions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.group_mentions ALTER COLUMN id SET DEFAULT nextval('public.group_mentions_id_seq'::regclass);


--
-- Name: group_requests id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.group_requests ALTER COLUMN id SET DEFAULT nextval('public.group_requests_id_seq'::regclass);


--
-- Name: group_tag_notification_defaults id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.group_tag_notification_defaults ALTER COLUMN id SET DEFAULT nextval('public.group_tag_notification_defaults_id_seq'::regclass);


--
-- Name: group_users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.group_users ALTER COLUMN id SET DEFAULT nextval('public.group_users_id_seq'::regclass);


--
-- Name: groups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.groups ALTER COLUMN id SET DEFAULT nextval('public.groups_id_seq'::regclass);


--
-- Name: ignored_users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ignored_users ALTER COLUMN id SET DEFAULT nextval('public.ignored_users_id_seq'::regclass);


--
-- Name: incoming_chat_webhooks id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.incoming_chat_webhooks ALTER COLUMN id SET DEFAULT nextval('public.incoming_chat_webhooks_id_seq'::regclass);


--
-- Name: incoming_domains id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.incoming_domains ALTER COLUMN id SET DEFAULT nextval('public.incoming_domains_id_seq'::regclass);


--
-- Name: incoming_emails id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.incoming_emails ALTER COLUMN id SET DEFAULT nextval('public.incoming_emails_id_seq'::regclass);


--
-- Name: incoming_links id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.incoming_links ALTER COLUMN id SET DEFAULT nextval('public.incoming_links_id_seq'::regclass);


--
-- Name: incoming_referers id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.incoming_referers ALTER COLUMN id SET DEFAULT nextval('public.incoming_referers_id_seq'::regclass);


--
-- Name: inferred_concepts id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.inferred_concepts ALTER COLUMN id SET DEFAULT nextval('public.inferred_concepts_id_seq'::regclass);


--
-- Name: invited_groups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invited_groups ALTER COLUMN id SET DEFAULT nextval('public.invited_groups_id_seq'::regclass);


--
-- Name: invited_users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invited_users ALTER COLUMN id SET DEFAULT nextval('public.invited_users_id_seq'::regclass);


--
-- Name: invites id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invites ALTER COLUMN id SET DEFAULT nextval('public.invites_id_seq'::regclass);


--
-- Name: javascript_caches id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.javascript_caches ALTER COLUMN id SET DEFAULT nextval('public.javascript_caches_id_seq'::regclass);


--
-- Name: linked_topics id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.linked_topics ALTER COLUMN id SET DEFAULT nextval('public.linked_topics_id_seq'::regclass);


--
-- Name: livestream_topic_chat_channels id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.livestream_topic_chat_channels ALTER COLUMN id SET DEFAULT nextval('public.livestream_topic_chat_channels_id_seq'::regclass);


--
-- Name: llm_credit_allocations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.llm_credit_allocations ALTER COLUMN id SET DEFAULT nextval('public.llm_credit_allocations_id_seq'::regclass);


--
-- Name: llm_credit_daily_usages id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.llm_credit_daily_usages ALTER COLUMN id SET DEFAULT nextval('public.llm_credit_daily_usages_id_seq'::regclass);


--
-- Name: llm_feature_credit_costs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.llm_feature_credit_costs ALTER COLUMN id SET DEFAULT nextval('public.llm_feature_credit_costs_id_seq'::regclass);


--
-- Name: llm_models id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.llm_models ALTER COLUMN id SET DEFAULT nextval('public.llm_models_id_seq'::regclass);


--
-- Name: llm_quota_usages id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.llm_quota_usages ALTER COLUMN id SET DEFAULT nextval('public.llm_quota_usages_id_seq'::regclass);


--
-- Name: llm_quotas id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.llm_quotas ALTER COLUMN id SET DEFAULT nextval('public.llm_quotas_id_seq'::regclass);


--
-- Name: message_bus id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_bus ALTER COLUMN id SET DEFAULT nextval('public.message_bus_id_seq'::regclass);


--
-- Name: model_accuracies id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.model_accuracies ALTER COLUMN id SET DEFAULT nextval('public.model_accuracies_id_seq'::regclass);


--
-- Name: moved_posts id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.moved_posts ALTER COLUMN id SET DEFAULT nextval('public.moved_posts_id_seq'::regclass);


--
-- Name: muted_users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.muted_users ALTER COLUMN id SET DEFAULT nextval('public.muted_users_id_seq'::regclass);


--
-- Name: nested_topics id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.nested_topics ALTER COLUMN id SET DEFAULT nextval('public.nested_topics_id_seq'::regclass);


--
-- Name: nested_view_post_stats id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.nested_view_post_stats ALTER COLUMN id SET DEFAULT nextval('public.nested_view_post_stats_id_seq'::regclass);


--
-- Name: notifications id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications ALTER COLUMN id SET DEFAULT nextval('public.notifications_id_seq'::regclass);


--
-- Name: oauth2_user_infos id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.oauth2_user_infos ALTER COLUMN id SET DEFAULT nextval('public.oauth2_user_infos_id_seq'::regclass);


--
-- Name: onceoff_logs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.onceoff_logs ALTER COLUMN id SET DEFAULT nextval('public.onceoff_logs_id_seq'::regclass);


--
-- Name: optimized_images id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.optimized_images ALTER COLUMN id SET DEFAULT nextval('public.optimized_images_id_seq'::regclass);


--
-- Name: optimized_videos id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.optimized_videos ALTER COLUMN id SET DEFAULT nextval('public.optimized_videos_id_seq'::regclass);


--
-- Name: permalinks id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permalinks ALTER COLUMN id SET DEFAULT nextval('public.permalinks_id_seq'::regclass);


--
-- Name: plugin_store_rows id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.plugin_store_rows ALTER COLUMN id SET DEFAULT nextval('public.plugin_store_rows_id_seq'::regclass);


--
-- Name: policy_users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.policy_users ALTER COLUMN id SET DEFAULT nextval('public.policy_users_id_seq'::regclass);


--
-- Name: poll_options id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.poll_options ALTER COLUMN id SET DEFAULT nextval('public.poll_options_id_seq'::regclass);


--
-- Name: polls id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.polls ALTER COLUMN id SET DEFAULT nextval('public.polls_id_seq'::regclass);


--
-- Name: post_action_types id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_action_types ALTER COLUMN id SET DEFAULT nextval('public.post_action_types_id_seq'::regclass);


--
-- Name: post_actions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_actions ALTER COLUMN id SET DEFAULT nextval('public.post_actions_id_seq'::regclass);


--
-- Name: post_custom_fields id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_custom_fields ALTER COLUMN id SET DEFAULT nextval('public.post_custom_fields_id_seq'::regclass);


--
-- Name: post_custom_prompts id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_custom_prompts ALTER COLUMN id SET DEFAULT nextval('public.post_custom_prompts_id_seq'::regclass);


--
-- Name: post_details id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_details ALTER COLUMN id SET DEFAULT nextval('public.post_details_id_seq'::regclass);


--
-- Name: post_hotlinked_media id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_hotlinked_media ALTER COLUMN id SET DEFAULT nextval('public.post_hotlinked_media_id_seq'::regclass);


--
-- Name: post_localizations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_localizations ALTER COLUMN id SET DEFAULT nextval('public.post_localizations_id_seq'::regclass);


--
-- Name: post_policies id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_policies ALTER COLUMN id SET DEFAULT nextval('public.post_policies_id_seq'::regclass);


--
-- Name: post_policy_groups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_policy_groups ALTER COLUMN id SET DEFAULT nextval('public.post_policy_groups_id_seq'::regclass);


--
-- Name: post_reply_keys id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_reply_keys ALTER COLUMN id SET DEFAULT nextval('public.post_reply_keys_id_seq'::regclass);


--
-- Name: post_revisions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_revisions ALTER COLUMN id SET DEFAULT nextval('public.post_revisions_id_seq'::regclass);


--
-- Name: post_stats id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_stats ALTER COLUMN id SET DEFAULT nextval('public.post_stats_id_seq'::regclass);


--
-- Name: post_voting_comment_custom_fields id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_voting_comment_custom_fields ALTER COLUMN id SET DEFAULT nextval('public.post_voting_comment_custom_fields_id_seq'::regclass);


--
-- Name: post_voting_comments id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_voting_comments ALTER COLUMN id SET DEFAULT nextval('public.post_voting_comments_id_seq'::regclass);


--
-- Name: post_voting_votes id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_voting_votes ALTER COLUMN id SET DEFAULT nextval('public.post_voting_votes_id_seq'::regclass);


--
-- Name: posts id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.posts ALTER COLUMN id SET DEFAULT nextval('public.posts_id_seq'::regclass);


--
-- Name: problem_check_trackers id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.problem_check_trackers ALTER COLUMN id SET DEFAULT nextval('public.problem_check_trackers_id_seq'::regclass);


--
-- Name: published_pages id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.published_pages ALTER COLUMN id SET DEFAULT nextval('public.published_pages_id_seq'::regclass);


--
-- Name: push_subscriptions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.push_subscriptions ALTER COLUMN id SET DEFAULT nextval('public.push_subscriptions_id_seq'::regclass);


--
-- Name: quoted_posts id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.quoted_posts ALTER COLUMN id SET DEFAULT nextval('public.quoted_posts_id_seq'::regclass);


--
-- Name: rag_document_fragments id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rag_document_fragments ALTER COLUMN id SET DEFAULT nextval('public.rag_document_fragments_id_seq'::regclass);


--
-- Name: rag_document_sources id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rag_document_sources ALTER COLUMN id SET DEFAULT nextval('public.rag_document_sources_id_seq'::regclass);


--
-- Name: redelivering_webhook_events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.redelivering_webhook_events ALTER COLUMN id SET DEFAULT nextval('public.redelivering_webhook_events_id_seq'::regclass);


--
-- Name: remote_themes id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.remote_themes ALTER COLUMN id SET DEFAULT nextval('public.remote_themes_id_seq'::regclass);


--
-- Name: reviewable_claimed_topics id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviewable_claimed_topics ALTER COLUMN id SET DEFAULT nextval('public.reviewable_claimed_topics_id_seq'::regclass);


--
-- Name: reviewable_histories id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviewable_histories ALTER COLUMN id SET DEFAULT nextval('public.reviewable_histories_id_seq'::regclass);


--
-- Name: reviewable_notes id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviewable_notes ALTER COLUMN id SET DEFAULT nextval('public.reviewable_notes_id_seq'::regclass);


--
-- Name: reviewable_scores id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviewable_scores ALTER COLUMN id SET DEFAULT nextval('public.reviewable_scores_id_seq'::regclass);


--
-- Name: reviewables id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviewables ALTER COLUMN id SET DEFAULT nextval('public.reviewables_id_seq'::regclass);


--
-- Name: scheduler_stats id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scheduler_stats ALTER COLUMN id SET DEFAULT nextval('public.scheduler_stats_id_seq'::regclass);


--
-- Name: schema_migration_details id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migration_details ALTER COLUMN id SET DEFAULT nextval('public.schema_migration_details_id_seq'::regclass);


--
-- Name: screened_emails id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.screened_emails ALTER COLUMN id SET DEFAULT nextval('public.screened_emails_id_seq'::regclass);


--
-- Name: screened_ip_addresses id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.screened_ip_addresses ALTER COLUMN id SET DEFAULT nextval('public.screened_ip_addresses_id_seq'::regclass);


--
-- Name: screened_urls id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.screened_urls ALTER COLUMN id SET DEFAULT nextval('public.screened_urls_id_seq'::regclass);


--
-- Name: search_logs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.search_logs ALTER COLUMN id SET DEFAULT nextval('public.search_logs_id_seq'::regclass);


--
-- Name: shared_ai_conversations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.shared_ai_conversations ALTER COLUMN id SET DEFAULT nextval('public.shared_ai_conversations_id_seq'::regclass);


--
-- Name: shared_drafts id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.shared_drafts ALTER COLUMN id SET DEFAULT nextval('public.shared_drafts_id_seq'::regclass);


--
-- Name: shelved_notifications id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.shelved_notifications ALTER COLUMN id SET DEFAULT nextval('public.shelved_notifications_id_seq'::regclass);


--
-- Name: sidebar_section_links id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sidebar_section_links ALTER COLUMN id SET DEFAULT nextval('public.sidebar_section_links_id_seq'::regclass);


--
-- Name: sidebar_section_localizations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sidebar_section_localizations ALTER COLUMN id SET DEFAULT nextval('public.sidebar_section_localizations_id_seq'::regclass);


--
-- Name: sidebar_sections id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sidebar_sections ALTER COLUMN id SET DEFAULT nextval('public.sidebar_sections_id_seq'::regclass);


--
-- Name: sidebar_url_localizations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sidebar_url_localizations ALTER COLUMN id SET DEFAULT nextval('public.sidebar_url_localizations_id_seq'::regclass);


--
-- Name: sidebar_urls id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sidebar_urls ALTER COLUMN id SET DEFAULT nextval('public.sidebar_urls_id_seq'::regclass);


--
-- Name: silenced_assignments id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.silenced_assignments ALTER COLUMN id SET DEFAULT nextval('public.silenced_assignments_id_seq'::regclass);


--
-- Name: single_sign_on_records id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.single_sign_on_records ALTER COLUMN id SET DEFAULT nextval('public.single_sign_on_records_id_seq'::regclass);


--
-- Name: site_setting_groups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.site_setting_groups ALTER COLUMN id SET DEFAULT nextval('public.site_setting_groups_id_seq'::regclass);


--
-- Name: site_setting_localizations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.site_setting_localizations ALTER COLUMN id SET DEFAULT nextval('public.site_setting_localizations_id_seq'::regclass);


--
-- Name: site_settings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.site_settings ALTER COLUMN id SET DEFAULT nextval('public.site_settings_id_seq'::regclass);


--
-- Name: sitemaps id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sitemaps ALTER COLUMN id SET DEFAULT nextval('public.sitemaps_id_seq'::regclass);


--
-- Name: skipped_email_logs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.skipped_email_logs ALTER COLUMN id SET DEFAULT nextval('public.skipped_email_logs_id_seq'::regclass);


--
-- Name: stylesheet_cache id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stylesheet_cache ALTER COLUMN id SET DEFAULT nextval('public.stylesheet_cache_id_seq'::regclass);


--
-- Name: summary_sections id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.summary_sections ALTER COLUMN id SET DEFAULT nextval('public.summary_sections_id_seq'::regclass);


--
-- Name: tag_group_memberships id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_group_memberships ALTER COLUMN id SET DEFAULT nextval('public.tag_group_memberships_id_seq'::regclass);


--
-- Name: tag_group_permissions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_group_permissions ALTER COLUMN id SET DEFAULT nextval('public.tag_group_permissions_id_seq'::regclass);


--
-- Name: tag_groups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_groups ALTER COLUMN id SET DEFAULT nextval('public.tag_groups_id_seq'::regclass);


--
-- Name: tag_localizations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_localizations ALTER COLUMN id SET DEFAULT nextval('public.tag_localizations_id_seq'::regclass);


--
-- Name: tag_search_data tag_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_search_data ALTER COLUMN tag_id SET DEFAULT nextval('public.tag_search_data_tag_id_seq'::regclass);


--
-- Name: tag_users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_users ALTER COLUMN id SET DEFAULT nextval('public.tag_users_id_seq'::regclass);


--
-- Name: tags id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tags ALTER COLUMN id SET DEFAULT nextval('public.tags_id_seq'::regclass);


--
-- Name: theme_fields id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.theme_fields ALTER COLUMN id SET DEFAULT nextval('public.theme_fields_id_seq'::regclass);


--
-- Name: theme_modifier_sets id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.theme_modifier_sets ALTER COLUMN id SET DEFAULT nextval('public.theme_modifier_sets_id_seq'::regclass);


--
-- Name: theme_settings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.theme_settings ALTER COLUMN id SET DEFAULT nextval('public.theme_settings_id_seq'::regclass);


--
-- Name: theme_settings_migrations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.theme_settings_migrations ALTER COLUMN id SET DEFAULT nextval('public.theme_settings_migrations_id_seq'::regclass);


--
-- Name: theme_site_settings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.theme_site_settings ALTER COLUMN id SET DEFAULT nextval('public.theme_site_settings_id_seq'::regclass);


--
-- Name: theme_svg_sprites id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.theme_svg_sprites ALTER COLUMN id SET DEFAULT nextval('public.theme_svg_sprites_id_seq'::regclass);


--
-- Name: theme_translation_overrides id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.theme_translation_overrides ALTER COLUMN id SET DEFAULT nextval('public.theme_translation_overrides_id_seq'::regclass);


--
-- Name: themes id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.themes ALTER COLUMN id SET DEFAULT nextval('public.themes_id_seq'::regclass);


--
-- Name: top_topics id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.top_topics ALTER COLUMN id SET DEFAULT nextval('public.top_topics_id_seq'::regclass);


--
-- Name: topic_allowed_groups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_allowed_groups ALTER COLUMN id SET DEFAULT nextval('public.topic_allowed_groups_id_seq'::regclass);


--
-- Name: topic_allowed_users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_allowed_users ALTER COLUMN id SET DEFAULT nextval('public.topic_allowed_users_id_seq'::regclass);


--
-- Name: topic_custom_fields id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_custom_fields ALTER COLUMN id SET DEFAULT nextval('public.topic_custom_fields_id_seq'::regclass);


--
-- Name: topic_embeds id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_embeds ALTER COLUMN id SET DEFAULT nextval('public.topic_embeds_id_seq'::regclass);


--
-- Name: topic_groups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_groups ALTER COLUMN id SET DEFAULT nextval('public.topic_groups_id_seq'::regclass);


--
-- Name: topic_hot_scores id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_hot_scores ALTER COLUMN id SET DEFAULT nextval('public.topic_hot_scores_id_seq'::regclass);


--
-- Name: topic_invites id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_invites ALTER COLUMN id SET DEFAULT nextval('public.topic_invites_id_seq'::regclass);


--
-- Name: topic_link_clicks id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_link_clicks ALTER COLUMN id SET DEFAULT nextval('public.topic_link_clicks_id_seq'::regclass);


--
-- Name: topic_links id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_links ALTER COLUMN id SET DEFAULT nextval('public.topic_links_id_seq'::regclass);


--
-- Name: topic_localizations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_localizations ALTER COLUMN id SET DEFAULT nextval('public.topic_localizations_id_seq'::regclass);


--
-- Name: topic_tags id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_tags ALTER COLUMN id SET DEFAULT nextval('public.topic_tags_id_seq'::regclass);


--
-- Name: topic_thumbnails id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_thumbnails ALTER COLUMN id SET DEFAULT nextval('public.topic_thumbnails_id_seq'::regclass);


--
-- Name: topic_timers id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_timers ALTER COLUMN id SET DEFAULT nextval('public.topic_timers_id_seq'::regclass);


--
-- Name: topic_users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_users ALTER COLUMN id SET DEFAULT nextval('public.topic_users_id_seq'::regclass);


--
-- Name: topic_view_stats id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_view_stats ALTER COLUMN id SET DEFAULT nextval('public.topic_view_stats_id_seq'::regclass);


--
-- Name: topic_voting_category_settings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_voting_category_settings ALTER COLUMN id SET DEFAULT nextval('public.topic_voting_category_settings_id_seq'::regclass);


--
-- Name: topic_voting_topic_vote_count id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_voting_topic_vote_count ALTER COLUMN id SET DEFAULT nextval('public.topic_voting_topic_vote_count_id_seq'::regclass);


--
-- Name: topic_voting_votes id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_voting_votes ALTER COLUMN id SET DEFAULT nextval('public.topic_voting_votes_id_seq'::regclass);


--
-- Name: topics id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topics ALTER COLUMN id SET DEFAULT nextval('public.topics_id_seq'::regclass);


--
-- Name: translation_overrides id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.translation_overrides ALTER COLUMN id SET DEFAULT nextval('public.translation_overrides_id_seq'::regclass);


--
-- Name: upcoming_change_events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.upcoming_change_events ALTER COLUMN id SET DEFAULT nextval('public.upcoming_change_events_id_seq'::regclass);


--
-- Name: upload_references id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.upload_references ALTER COLUMN id SET DEFAULT nextval('public.upload_references_id_seq'::regclass);


--
-- Name: uploads id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.uploads ALTER COLUMN id SET DEFAULT nextval('public.uploads_id_seq'::regclass);


--
-- Name: user_actions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_actions ALTER COLUMN id SET DEFAULT nextval('public.user_actions_id_seq'::regclass);


--
-- Name: user_api_key_client_scopes id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_api_key_client_scopes ALTER COLUMN id SET DEFAULT nextval('public.user_api_key_client_scopes_id_seq'::regclass);


--
-- Name: user_api_key_clients id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_api_key_clients ALTER COLUMN id SET DEFAULT nextval('public.user_api_key_clients_id_seq'::regclass);


--
-- Name: user_api_key_scopes id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_api_key_scopes ALTER COLUMN id SET DEFAULT nextval('public.user_api_key_scopes_id_seq'::regclass);


--
-- Name: user_api_keys id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_api_keys ALTER COLUMN id SET DEFAULT nextval('public.user_api_keys_id_seq'::regclass);


--
-- Name: user_archived_messages id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_archived_messages ALTER COLUMN id SET DEFAULT nextval('public.user_archived_messages_id_seq'::regclass);


--
-- Name: user_associated_accounts id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_associated_accounts ALTER COLUMN id SET DEFAULT nextval('public.user_associated_accounts_id_seq'::regclass);


--
-- Name: user_associated_groups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_associated_groups ALTER COLUMN id SET DEFAULT nextval('public.user_associated_groups_id_seq'::regclass);


--
-- Name: user_auth_token_logs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_auth_token_logs ALTER COLUMN id SET DEFAULT nextval('public.user_auth_token_logs_id_seq'::regclass);


--
-- Name: user_auth_tokens id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_auth_tokens ALTER COLUMN id SET DEFAULT nextval('public.user_auth_tokens_id_seq'::regclass);


--
-- Name: user_avatars id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_avatars ALTER COLUMN id SET DEFAULT nextval('public.user_avatars_id_seq'::regclass);


--
-- Name: user_badges id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_badges ALTER COLUMN id SET DEFAULT nextval('public.user_badges_id_seq'::regclass);


--
-- Name: user_chat_channel_memberships id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_chat_channel_memberships ALTER COLUMN id SET DEFAULT nextval('public.user_chat_channel_memberships_id_seq'::regclass);


--
-- Name: user_chat_thread_memberships id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_chat_thread_memberships ALTER COLUMN id SET DEFAULT nextval('public.user_chat_thread_memberships_id_seq'::regclass);


--
-- Name: user_custom_fields id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_custom_fields ALTER COLUMN id SET DEFAULT nextval('public.user_custom_fields_id_seq'::regclass);


--
-- Name: user_emails id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_emails ALTER COLUMN id SET DEFAULT nextval('public.user_emails_id_seq'::regclass);


--
-- Name: user_exports id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_exports ALTER COLUMN id SET DEFAULT nextval('public.user_exports_id_seq'::regclass);


--
-- Name: user_field_options id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_field_options ALTER COLUMN id SET DEFAULT nextval('public.user_field_options_id_seq'::regclass);


--
-- Name: user_fields id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_fields ALTER COLUMN id SET DEFAULT nextval('public.user_fields_id_seq'::regclass);


--
-- Name: user_histories id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_histories ALTER COLUMN id SET DEFAULT nextval('public.user_histories_id_seq'::regclass);


--
-- Name: user_ip_address_histories id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_ip_address_histories ALTER COLUMN id SET DEFAULT nextval('public.user_ip_address_histories_id_seq'::regclass);


--
-- Name: user_notification_schedules id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_notification_schedules ALTER COLUMN id SET DEFAULT nextval('public.user_notification_schedules_id_seq'::regclass);


--
-- Name: user_open_ids id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_open_ids ALTER COLUMN id SET DEFAULT nextval('public.user_open_ids_id_seq'::regclass);


--
-- Name: user_passwords id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_passwords ALTER COLUMN id SET DEFAULT nextval('public.user_passwords_id_seq'::regclass);


--
-- Name: user_profile_views id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_profile_views ALTER COLUMN id SET DEFAULT nextval('public.user_profile_views_id_seq'::regclass);


--
-- Name: user_required_fields_versions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_required_fields_versions ALTER COLUMN id SET DEFAULT nextval('public.user_required_fields_versions_id_seq'::regclass);


--
-- Name: user_second_factors id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_second_factors ALTER COLUMN id SET DEFAULT nextval('public.user_second_factors_id_seq'::regclass);


--
-- Name: user_security_keys id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_security_keys ALTER COLUMN id SET DEFAULT nextval('public.user_security_keys_id_seq'::regclass);


--
-- Name: user_statuses user_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_statuses ALTER COLUMN user_id SET DEFAULT nextval('public.user_statuses_user_id_seq'::regclass);


--
-- Name: user_uploads id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_uploads ALTER COLUMN id SET DEFAULT nextval('public.user_uploads_id_seq'::regclass);


--
-- Name: user_visit_daily_rollups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_visit_daily_rollups ALTER COLUMN id SET DEFAULT nextval('public.user_visit_daily_rollups_id_seq'::regclass);


--
-- Name: user_visits id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_visits ALTER COLUMN id SET DEFAULT nextval('public.user_visits_id_seq'::regclass);


--
-- Name: user_warnings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_warnings ALTER COLUMN id SET DEFAULT nextval('public.user_warnings_id_seq'::regclass);


--
-- Name: users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users ALTER COLUMN id SET DEFAULT nextval('public.users_id_seq'::regclass);


--
-- Name: voice_co_presences id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.voice_co_presences ALTER COLUMN id SET DEFAULT nextval('public.voice_co_presences_id_seq'::regclass);


--
-- Name: voice_invites id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.voice_invites ALTER COLUMN id SET DEFAULT nextval('public.voice_invites_id_seq'::regclass);


--
-- Name: voice_recordings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.voice_recordings ALTER COLUMN id SET DEFAULT nextval('public.voice_recordings_id_seq'::regclass);


--
-- Name: voice_room_memberships id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.voice_room_memberships ALTER COLUMN id SET DEFAULT nextval('public.voice_room_memberships_id_seq'::regclass);


--
-- Name: voice_rooms id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.voice_rooms ALTER COLUMN id SET DEFAULT nextval('public.voice_rooms_id_seq'::regclass);


--
-- Name: voice_sessions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.voice_sessions ALTER COLUMN id SET DEFAULT nextval('public.voice_sessions_id_seq'::regclass);


--
-- Name: watched_word_groups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.watched_word_groups ALTER COLUMN id SET DEFAULT nextval('public.watched_word_groups_id_seq'::regclass);


--
-- Name: watched_words id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.watched_words ALTER COLUMN id SET DEFAULT nextval('public.watched_words_id_seq'::regclass);


--
-- Name: web_crawler_requests id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.web_crawler_requests ALTER COLUMN id SET DEFAULT nextval('public.web_crawler_requests_id_seq'::regclass);


--
-- Name: web_hook_event_types id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.web_hook_event_types ALTER COLUMN id SET DEFAULT nextval('public.web_hook_event_types_id_seq'::regclass);


--
-- Name: web_hook_events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.web_hook_events ALTER COLUMN id SET DEFAULT nextval('public.web_hook_events_id_seq'::regclass);


--
-- Name: web_hook_events_daily_aggregates id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.web_hook_events_daily_aggregates ALTER COLUMN id SET DEFAULT nextval('public.web_hook_events_daily_aggregates_id_seq'::regclass);


--
-- Name: web_hooks id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.web_hooks ALTER COLUMN id SET DEFAULT nextval('public.web_hooks_id_seq'::regclass);


--
-- Name: access_control_lists access_control_lists_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.access_control_lists
    ADD CONSTRAINT access_control_lists_pkey PRIMARY KEY (id);


--
-- Name: ad_plugin_house_ads ad_plugin_house_ads_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ad_plugin_house_ads
    ADD CONSTRAINT ad_plugin_house_ads_pkey PRIMARY KEY (id);


--
-- Name: ad_plugin_impressions ad_plugin_impressions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ad_plugin_impressions
    ADD CONSTRAINT ad_plugin_impressions_pkey PRIMARY KEY (id);


--
-- Name: admin_dashboard_reports admin_dashboard_reports_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.admin_dashboard_reports
    ADD CONSTRAINT admin_dashboard_reports_pkey PRIMARY KEY (id);


--
-- Name: admin_dashboard_sections admin_dashboard_sections_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.admin_dashboard_sections
    ADD CONSTRAINT admin_dashboard_sections_pkey PRIMARY KEY (id);


--
-- Name: admin_notices admin_notices_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.admin_notices
    ADD CONSTRAINT admin_notices_pkey PRIMARY KEY (id);


--
-- Name: ai_agent_mcp_servers ai_agent_mcp_servers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_agent_mcp_servers
    ADD CONSTRAINT ai_agent_mcp_servers_pkey PRIMARY KEY (id);


--
-- Name: ai_agents ai_agents_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_agents
    ADD CONSTRAINT ai_agents_pkey PRIMARY KEY (id);


--
-- Name: ai_api_audit_logs ai_api_audit_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_api_audit_logs
    ADD CONSTRAINT ai_api_audit_logs_pkey PRIMARY KEY (id);


--
-- Name: ai_api_request_stats ai_api_request_stats_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_api_request_stats
    ADD CONSTRAINT ai_api_request_stats_pkey PRIMARY KEY (id);


--
-- Name: ai_artifact_key_values ai_artifact_key_values_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_artifact_key_values
    ADD CONSTRAINT ai_artifact_key_values_pkey PRIMARY KEY (id);


--
-- Name: ai_artifact_versions ai_artifact_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_artifact_versions
    ADD CONSTRAINT ai_artifact_versions_pkey PRIMARY KEY (id);


--
-- Name: ai_artifacts ai_artifacts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_artifacts
    ADD CONSTRAINT ai_artifacts_pkey PRIMARY KEY (id);


--
-- Name: ai_mcp_oauth_tokens ai_mcp_oauth_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_mcp_oauth_tokens
    ADD CONSTRAINT ai_mcp_oauth_tokens_pkey PRIMARY KEY (id);


--
-- Name: ai_mcp_servers ai_mcp_servers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_mcp_servers
    ADD CONSTRAINT ai_mcp_servers_pkey PRIMARY KEY (id);


--
-- Name: ai_moderation_settings ai_moderation_settings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_moderation_settings
    ADD CONSTRAINT ai_moderation_settings_pkey PRIMARY KEY (id);


--
-- Name: ai_post_image_captions ai_post_image_captions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_post_image_captions
    ADD CONSTRAINT ai_post_image_captions_pkey PRIMARY KEY (id);


--
-- Name: ai_secrets ai_secrets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_secrets
    ADD CONSTRAINT ai_secrets_pkey PRIMARY KEY (id);


--
-- Name: ai_spam_logs ai_spam_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_spam_logs
    ADD CONSTRAINT ai_spam_logs_pkey PRIMARY KEY (id);


--
-- Name: ai_summaries ai_summaries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_summaries
    ADD CONSTRAINT ai_summaries_pkey PRIMARY KEY (id);


--
-- Name: ai_tool_actions ai_tool_actions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_tool_actions
    ADD CONSTRAINT ai_tool_actions_pkey PRIMARY KEY (id);


--
-- Name: ai_tool_secret_bindings ai_tool_secret_bindings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_tool_secret_bindings
    ADD CONSTRAINT ai_tool_secret_bindings_pkey PRIMARY KEY (id);


--
-- Name: ai_tools ai_tools_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_tools
    ADD CONSTRAINT ai_tools_pkey PRIMARY KEY (id);


--
-- Name: allowed_pm_users allowed_pm_users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.allowed_pm_users
    ADD CONSTRAINT allowed_pm_users_pkey PRIMARY KEY (id);


--
-- Name: anonymous_users anonymous_users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.anonymous_users
    ADD CONSTRAINT anonymous_users_pkey PRIMARY KEY (id);


--
-- Name: api_key_scopes api_key_scopes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.api_key_scopes
    ADD CONSTRAINT api_key_scopes_pkey PRIMARY KEY (id);


--
-- Name: api_keys api_keys_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.api_keys
    ADD CONSTRAINT api_keys_pkey PRIMARY KEY (id);


--
-- Name: application_requests application_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.application_requests
    ADD CONSTRAINT application_requests_pkey PRIMARY KEY (id);


--
-- Name: ar_internal_metadata ar_internal_metadata_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ar_internal_metadata
    ADD CONSTRAINT ar_internal_metadata_pkey PRIMARY KEY (key);


--
-- Name: ask_ai_logs ask_ai_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ask_ai_logs
    ADD CONSTRAINT ask_ai_logs_pkey PRIMARY KEY (id);


--
-- Name: assignments assignments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.assignments
    ADD CONSTRAINT assignments_pkey PRIMARY KEY (id);


--
-- Name: associated_groups associated_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.associated_groups
    ADD CONSTRAINT associated_groups_pkey PRIMARY KEY (id);


--
-- Name: backup_draft_posts backup_draft_posts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.backup_draft_posts
    ADD CONSTRAINT backup_draft_posts_pkey PRIMARY KEY (id);


--
-- Name: backup_draft_topics backup_draft_topics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.backup_draft_topics
    ADD CONSTRAINT backup_draft_topics_pkey PRIMARY KEY (id);


--
-- Name: backup_metadata backup_metadata_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.backup_metadata
    ADD CONSTRAINT backup_metadata_pkey PRIMARY KEY (id);


--
-- Name: badge_groupings badge_groupings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.badge_groupings
    ADD CONSTRAINT badge_groupings_pkey PRIMARY KEY (id);


--
-- Name: badge_types badge_types_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.badge_types
    ADD CONSTRAINT badge_types_pkey PRIMARY KEY (id);


--
-- Name: badges badges_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.badges
    ADD CONSTRAINT badges_pkey PRIMARY KEY (id);


--
-- Name: bookmarks bookmarks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bookmarks
    ADD CONSTRAINT bookmarks_pkey PRIMARY KEY (id);


--
-- Name: browser_pageview_country_daily_rollups browser_pageview_country_daily_rollups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.browser_pageview_country_daily_rollups
    ADD CONSTRAINT browser_pageview_country_daily_rollups_pkey PRIMARY KEY (id);


--
-- Name: browser_pageview_crawler_daily_rollups browser_pageview_crawler_daily_rollups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.browser_pageview_crawler_daily_rollups
    ADD CONSTRAINT browser_pageview_crawler_daily_rollups_pkey PRIMARY KEY (id);


--
-- Name: browser_pageview_entry_url_daily_rollups browser_pageview_entry_url_daily_rollups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.browser_pageview_entry_url_daily_rollups
    ADD CONSTRAINT browser_pageview_entry_url_daily_rollups_pkey PRIMARY KEY (id);


--
-- Name: browser_pageview_event_scores browser_pageview_event_scores_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.browser_pageview_event_scores
    ADD CONSTRAINT browser_pageview_event_scores_pkey PRIMARY KEY (id);


--
-- Name: browser_pageview_events browser_pageview_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.browser_pageview_events
    ADD CONSTRAINT browser_pageview_events_pkey PRIMARY KEY (id);


--
-- Name: browser_pageview_referrer_daily_rollups browser_pageview_referrer_daily_rollups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.browser_pageview_referrer_daily_rollups
    ADD CONSTRAINT browser_pageview_referrer_daily_rollups_pkey PRIMARY KEY (id);


--
-- Name: browser_pageview_session_engagement_daily_rollups browser_pageview_session_engagement_daily_rollups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.browser_pageview_session_engagement_daily_rollups
    ADD CONSTRAINT browser_pageview_session_engagement_daily_rollups_pkey PRIMARY KEY (id);


--
-- Name: browser_pageview_session_engagements browser_pageview_session_engagements_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.browser_pageview_session_engagements
    ADD CONSTRAINT browser_pageview_session_engagements_pkey PRIMARY KEY (id);


--
-- Name: calendar_events calendar_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.calendar_events
    ADD CONSTRAINT calendar_events_pkey PRIMARY KEY (id);


--
-- Name: categories categories_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.categories
    ADD CONSTRAINT categories_pkey PRIMARY KEY (id);


--
-- Name: category_search_data categories_search_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_search_data
    ADD CONSTRAINT categories_search_pkey PRIMARY KEY (category_id);


--
-- Name: category_activity_daily_rollups category_activity_daily_rollups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_activity_daily_rollups
    ADD CONSTRAINT category_activity_daily_rollups_pkey PRIMARY KEY (id);


--
-- Name: category_custom_fields category_custom_fields_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_custom_fields
    ADD CONSTRAINT category_custom_fields_pkey PRIMARY KEY (id);


--
-- Name: category_featured_topics category_featured_topics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_featured_topics
    ADD CONSTRAINT category_featured_topics_pkey PRIMARY KEY (id);


--
-- Name: category_form_templates category_form_templates_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_form_templates
    ADD CONSTRAINT category_form_templates_pkey PRIMARY KEY (id);


--
-- Name: category_groups category_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_groups
    ADD CONSTRAINT category_groups_pkey PRIMARY KEY (id);


--
-- Name: category_localizations category_localizations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_localizations
    ADD CONSTRAINT category_localizations_pkey PRIMARY KEY (id);


--
-- Name: category_moderation_groups category_moderation_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_moderation_groups
    ADD CONSTRAINT category_moderation_groups_pkey PRIMARY KEY (id);


--
-- Name: category_posting_review_groups category_posting_review_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_posting_review_groups
    ADD CONSTRAINT category_posting_review_groups_pkey PRIMARY KEY (id);


--
-- Name: category_required_tag_groups category_required_tag_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_required_tag_groups
    ADD CONSTRAINT category_required_tag_groups_pkey PRIMARY KEY (id);


--
-- Name: category_settings category_settings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_settings
    ADD CONSTRAINT category_settings_pkey PRIMARY KEY (id);


--
-- Name: category_tag_groups category_tag_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_tag_groups
    ADD CONSTRAINT category_tag_groups_pkey PRIMARY KEY (id);


--
-- Name: category_tag_stats category_tag_stats_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_tag_stats
    ADD CONSTRAINT category_tag_stats_pkey PRIMARY KEY (id);


--
-- Name: category_tags category_tags_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_tags
    ADD CONSTRAINT category_tags_pkey PRIMARY KEY (id);


--
-- Name: category_users category_users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.category_users
    ADD CONSTRAINT category_users_pkey PRIMARY KEY (id);


--
-- Name: chat_channel_archives chat_channel_archives_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_channel_archives
    ADD CONSTRAINT chat_channel_archives_pkey PRIMARY KEY (id);


--
-- Name: chat_channel_custom_fields chat_channel_custom_fields_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_channel_custom_fields
    ADD CONSTRAINT chat_channel_custom_fields_pkey PRIMARY KEY (id);


--
-- Name: chat_channels chat_channels_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_channels
    ADD CONSTRAINT chat_channels_pkey PRIMARY KEY (id);


--
-- Name: chat_drafts chat_drafts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_drafts
    ADD CONSTRAINT chat_drafts_pkey PRIMARY KEY (id);


--
-- Name: chat_mentions chat_mentions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_mentions
    ADD CONSTRAINT chat_mentions_pkey PRIMARY KEY (id);


--
-- Name: chat_message_custom_fields chat_message_custom_fields_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_custom_fields
    ADD CONSTRAINT chat_message_custom_fields_pkey PRIMARY KEY (id);


--
-- Name: chat_message_custom_prompts chat_message_custom_prompts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_custom_prompts
    ADD CONSTRAINT chat_message_custom_prompts_pkey PRIMARY KEY (id);


--
-- Name: chat_message_hotlinked_media chat_message_hotlinked_media_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_hotlinked_media
    ADD CONSTRAINT chat_message_hotlinked_media_pkey PRIMARY KEY (id);


--
-- Name: chat_message_interactions chat_message_interactions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_interactions
    ADD CONSTRAINT chat_message_interactions_pkey PRIMARY KEY (id);


--
-- Name: chat_message_links chat_message_links_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_links
    ADD CONSTRAINT chat_message_links_pkey PRIMARY KEY (id);


--
-- Name: chat_message_reactions chat_message_reactions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_reactions
    ADD CONSTRAINT chat_message_reactions_pkey PRIMARY KEY (id);


--
-- Name: chat_message_revisions chat_message_revisions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_revisions
    ADD CONSTRAINT chat_message_revisions_pkey PRIMARY KEY (id);


--
-- Name: chat_message_search_data chat_message_search_data_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message_search_data
    ADD CONSTRAINT chat_message_search_data_pkey PRIMARY KEY (chat_message_id);


--
-- Name: chat_messages chat_messages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_messages
    ADD CONSTRAINT chat_messages_pkey PRIMARY KEY (id);


--
-- Name: chat_pinned_messages chat_pinned_messages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_pinned_messages
    ADD CONSTRAINT chat_pinned_messages_pkey PRIMARY KEY (id);


--
-- Name: chat_thread_custom_fields chat_thread_custom_fields_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_thread_custom_fields
    ADD CONSTRAINT chat_thread_custom_fields_pkey PRIMARY KEY (id);


--
-- Name: chat_threads chat_threads_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_threads
    ADD CONSTRAINT chat_threads_pkey PRIMARY KEY (id);


--
-- Name: chat_webhook_events chat_webhook_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_webhook_events
    ADD CONSTRAINT chat_webhook_events_pkey PRIMARY KEY (id);


--
-- Name: child_themes child_themes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.child_themes
    ADD CONSTRAINT child_themes_pkey PRIMARY KEY (id);


--
-- Name: classification_results classification_results_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.classification_results
    ADD CONSTRAINT classification_results_pkey PRIMARY KEY (id);


--
-- Name: color_scheme_colors color_scheme_colors_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.color_scheme_colors
    ADD CONSTRAINT color_scheme_colors_pkey PRIMARY KEY (id);


--
-- Name: color_schemes color_schemes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.color_schemes
    ADD CONSTRAINT color_schemes_pkey PRIMARY KEY (id);


--
-- Name: completion_prompts completion_prompts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.completion_prompts
    ADD CONSTRAINT completion_prompts_pkey PRIMARY KEY (id);


--
-- Name: custom_emojis custom_emojis_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.custom_emojis
    ADD CONSTRAINT custom_emojis_pkey PRIMARY KEY (id);


--
-- Name: data_explorer_queries data_explorer_queries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.data_explorer_queries
    ADD CONSTRAINT data_explorer_queries_pkey PRIMARY KEY (id);


--
-- Name: data_explorer_query_groups data_explorer_query_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.data_explorer_query_groups
    ADD CONSTRAINT data_explorer_query_groups_pkey PRIMARY KEY (id);


--
-- Name: data_explorer_query_stats data_explorer_query_stats_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.data_explorer_query_stats
    ADD CONSTRAINT data_explorer_query_stats_pkey PRIMARY KEY (id);


--
-- Name: developers developers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.developers
    ADD CONSTRAINT developers_pkey PRIMARY KEY (id);


--
-- Name: unsubscribe_keys digest_unsubscribe_keys_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.unsubscribe_keys
    ADD CONSTRAINT digest_unsubscribe_keys_pkey PRIMARY KEY (key);


--
-- Name: direct_message_channels direct_message_channels_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.direct_message_channels
    ADD CONSTRAINT direct_message_channels_pkey PRIMARY KEY (id);


--
-- Name: direct_message_users direct_message_users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.direct_message_users
    ADD CONSTRAINT direct_message_users_pkey PRIMARY KEY (id);


--
-- Name: directory_columns directory_columns_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.directory_columns
    ADD CONSTRAINT directory_columns_pkey PRIMARY KEY (id);


--
-- Name: directory_items directory_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.directory_items
    ADD CONSTRAINT directory_items_pkey PRIMARY KEY (id);


--
-- Name: discourse_ai_ai_bot_conversation_stars discourse_ai_ai_bot_conversation_stars_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_ai_ai_bot_conversation_stars
    ADD CONSTRAINT discourse_ai_ai_bot_conversation_stars_pkey PRIMARY KEY (id);


--
-- Name: discourse_automation_automations discourse_automation_automations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_automation_automations
    ADD CONSTRAINT discourse_automation_automations_pkey PRIMARY KEY (id);


--
-- Name: discourse_automation_fields discourse_automation_fields_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_automation_fields
    ADD CONSTRAINT discourse_automation_fields_pkey PRIMARY KEY (id);


--
-- Name: discourse_automation_pending_automations discourse_automation_pending_automations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_automation_pending_automations
    ADD CONSTRAINT discourse_automation_pending_automations_pkey PRIMARY KEY (id);


--
-- Name: discourse_automation_pending_pms discourse_automation_pending_pms_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_automation_pending_pms
    ADD CONSTRAINT discourse_automation_pending_pms_pkey PRIMARY KEY (id);


--
-- Name: discourse_automation_stats discourse_automation_stats_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_automation_stats
    ADD CONSTRAINT discourse_automation_stats_pkey PRIMARY KEY (id);


--
-- Name: discourse_automation_user_global_notices discourse_automation_user_global_notices_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_automation_user_global_notices
    ADD CONSTRAINT discourse_automation_user_global_notices_pkey PRIMARY KEY (id);


--
-- Name: discourse_calendar_disabled_holidays discourse_calendar_disabled_holidays_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_calendar_disabled_holidays
    ADD CONSTRAINT discourse_calendar_disabled_holidays_pkey PRIMARY KEY (id);


--
-- Name: discourse_calendar_post_event_dates discourse_calendar_post_event_dates_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_calendar_post_event_dates
    ADD CONSTRAINT discourse_calendar_post_event_dates_pkey PRIMARY KEY (id);


--
-- Name: discourse_kanban_board_histories discourse_kanban_board_histories_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_kanban_board_histories
    ADD CONSTRAINT discourse_kanban_board_histories_pkey PRIMARY KEY (id);


--
-- Name: discourse_kanban_boards discourse_kanban_boards_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_kanban_boards
    ADD CONSTRAINT discourse_kanban_boards_pkey PRIMARY KEY (id);


--
-- Name: discourse_kanban_card_histories discourse_kanban_card_histories_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_kanban_card_histories
    ADD CONSTRAINT discourse_kanban_card_histories_pkey PRIMARY KEY (id);


--
-- Name: discourse_kanban_cards discourse_kanban_cards_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_kanban_cards
    ADD CONSTRAINT discourse_kanban_cards_pkey PRIMARY KEY (id);


--
-- Name: discourse_kanban_columns discourse_kanban_columns_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_kanban_columns
    ADD CONSTRAINT discourse_kanban_columns_pkey PRIMARY KEY (id);


--
-- Name: discourse_post_event_events discourse_post_event_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_post_event_events
    ADD CONSTRAINT discourse_post_event_events_pkey PRIMARY KEY (id);


--
-- Name: discourse_post_event_hosts discourse_post_event_hosts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_post_event_hosts
    ADD CONSTRAINT discourse_post_event_hosts_pkey PRIMARY KEY (id);


--
-- Name: discourse_post_event_invitees discourse_post_event_invitees_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_post_event_invitees
    ADD CONSTRAINT discourse_post_event_invitees_pkey PRIMARY KEY (id);


--
-- Name: discourse_reactions_reaction_users discourse_reactions_reaction_users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_reactions_reaction_users
    ADD CONSTRAINT discourse_reactions_reaction_users_pkey PRIMARY KEY (id);


--
-- Name: discourse_reactions_reactions discourse_reactions_reactions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_reactions_reactions
    ADD CONSTRAINT discourse_reactions_reactions_pkey PRIMARY KEY (id);


--
-- Name: discourse_rss_polling_poll_attempts discourse_rss_polling_poll_attempts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_rss_polling_poll_attempts
    ADD CONSTRAINT discourse_rss_polling_poll_attempts_pkey PRIMARY KEY (id);


--
-- Name: discourse_rss_polling_rss_feeds discourse_rss_polling_rss_feeds_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_rss_polling_rss_feeds
    ADD CONSTRAINT discourse_rss_polling_rss_feeds_pkey PRIMARY KEY (id);


--
-- Name: discourse_solved_shared_issues discourse_solved_shared_issues_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_solved_shared_issues
    ADD CONSTRAINT discourse_solved_shared_issues_pkey PRIMARY KEY (id);


--
-- Name: discourse_solved_solved_topics discourse_solved_solved_topics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_solved_solved_topics
    ADD CONSTRAINT discourse_solved_solved_topics_pkey PRIMARY KEY (id);


--
-- Name: discourse_solved_topic_answers discourse_solved_topic_answers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_solved_topic_answers
    ADD CONSTRAINT discourse_solved_topic_answers_pkey PRIMARY KEY (id);


--
-- Name: discourse_subscriptions_customers discourse_subscriptions_customers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_subscriptions_customers
    ADD CONSTRAINT discourse_subscriptions_customers_pkey PRIMARY KEY (id);


--
-- Name: discourse_subscriptions_products discourse_subscriptions_products_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_subscriptions_products
    ADD CONSTRAINT discourse_subscriptions_products_pkey PRIMARY KEY (id);


--
-- Name: discourse_subscriptions_subscriptions discourse_subscriptions_subscriptions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_subscriptions_subscriptions
    ADD CONSTRAINT discourse_subscriptions_subscriptions_pkey PRIMARY KEY (id);


--
-- Name: discourse_templates_usage_count discourse_templates_usage_count_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_templates_usage_count
    ADD CONSTRAINT discourse_templates_usage_count_pkey PRIMARY KEY (id);


--
-- Name: discourse_workflows_ai_authoring_sessions discourse_workflows_ai_authoring_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_ai_authoring_sessions
    ADD CONSTRAINT discourse_workflows_ai_authoring_sessions_pkey PRIMARY KEY (id);


--
-- Name: discourse_workflows_credentials discourse_workflows_credentials_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_credentials
    ADD CONSTRAINT discourse_workflows_credentials_pkey PRIMARY KEY (id);


--
-- Name: discourse_workflows_data_tables discourse_workflows_data_tables_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_data_tables
    ADD CONSTRAINT discourse_workflows_data_tables_pkey PRIMARY KEY (id);


--
-- Name: discourse_workflows_execution_stats discourse_workflows_execution_stats_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_execution_stats
    ADD CONSTRAINT discourse_workflows_execution_stats_pkey PRIMARY KEY (id);


--
-- Name: discourse_workflows_executions discourse_workflows_executions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_executions
    ADD CONSTRAINT discourse_workflows_executions_pkey PRIMARY KEY (id);


--
-- Name: discourse_workflows_tags discourse_workflows_tags_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_tags
    ADD CONSTRAINT discourse_workflows_tags_pkey PRIMARY KEY (id);


--
-- Name: discourse_workflows_variables discourse_workflows_variables_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_variables
    ADD CONSTRAINT discourse_workflows_variables_pkey PRIMARY KEY (id);


--
-- Name: discourse_workflows_webhooks discourse_workflows_webhooks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_webhooks
    ADD CONSTRAINT discourse_workflows_webhooks_pkey PRIMARY KEY (id);


--
-- Name: discourse_workflows_workflow_call_runs discourse_workflows_workflow_call_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_workflow_call_runs
    ADD CONSTRAINT discourse_workflows_workflow_call_runs_pkey PRIMARY KEY (id);


--
-- Name: discourse_workflows_workflow_dependencies discourse_workflows_workflow_dependencies_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_workflow_dependencies
    ADD CONSTRAINT discourse_workflows_workflow_dependencies_pkey PRIMARY KEY (id);


--
-- Name: discourse_workflows_workflow_publish_history discourse_workflows_workflow_publish_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_workflow_publish_history
    ADD CONSTRAINT discourse_workflows_workflow_publish_history_pkey PRIMARY KEY (id);


--
-- Name: discourse_workflows_workflow_tags discourse_workflows_workflow_tags_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_workflow_tags
    ADD CONSTRAINT discourse_workflows_workflow_tags_pkey PRIMARY KEY (id);


--
-- Name: discourse_workflows_workflow_versions discourse_workflows_workflow_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_workflow_versions
    ADD CONSTRAINT discourse_workflows_workflow_versions_pkey PRIMARY KEY (version_id);


--
-- Name: discourse_workflows_workflows discourse_workflows_workflows_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_workflows_workflows
    ADD CONSTRAINT discourse_workflows_workflows_pkey PRIMARY KEY (id);


--
-- Name: dismissed_topic_users dismissed_topic_users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dismissed_topic_users
    ADD CONSTRAINT dismissed_topic_users_pkey PRIMARY KEY (id);


--
-- Name: do_not_disturb_timings do_not_disturb_timings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.do_not_disturb_timings
    ADD CONSTRAINT do_not_disturb_timings_pkey PRIMARY KEY (id);


--
-- Name: draft_sequences draft_sequences_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.draft_sequences
    ADD CONSTRAINT draft_sequences_pkey PRIMARY KEY (id);


--
-- Name: drafts drafts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drafts
    ADD CONSTRAINT drafts_pkey PRIMARY KEY (id);


--
-- Name: email_change_requests email_change_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.email_change_requests
    ADD CONSTRAINT email_change_requests_pkey PRIMARY KEY (id);


--
-- Name: email_login_codes email_login_codes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.email_login_codes
    ADD CONSTRAINT email_login_codes_pkey PRIMARY KEY (id);


--
-- Name: email_logs email_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.email_logs
    ADD CONSTRAINT email_logs_pkey PRIMARY KEY (id);


--
-- Name: email_tokens email_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.email_tokens
    ADD CONSTRAINT email_tokens_pkey PRIMARY KEY (id);


--
-- Name: embeddable_host_tags embeddable_host_tags_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.embeddable_host_tags
    ADD CONSTRAINT embeddable_host_tags_pkey PRIMARY KEY (id);


--
-- Name: embeddable_hosts embeddable_hosts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.embeddable_hosts
    ADD CONSTRAINT embeddable_hosts_pkey PRIMARY KEY (id);


--
-- Name: embedding_definitions embedding_definitions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.embedding_definitions
    ADD CONSTRAINT embedding_definitions_pkey PRIMARY KEY (id);


--
-- Name: external_upload_stubs external_upload_stubs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.external_upload_stubs
    ADD CONSTRAINT external_upload_stubs_pkey PRIMARY KEY (id);


--
-- Name: flags flags_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.flags
    ADD CONSTRAINT flags_pkey PRIMARY KEY (id);


--
-- Name: form_templates form_templates_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.form_templates
    ADD CONSTRAINT form_templates_pkey PRIMARY KEY (id);


--
-- Name: gamification_leaderboard_scores gamification_leaderboard_scores_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gamification_leaderboard_scores
    ADD CONSTRAINT gamification_leaderboard_scores_pkey PRIMARY KEY (id);


--
-- Name: gamification_leaderboards gamification_leaderboards_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gamification_leaderboards
    ADD CONSTRAINT gamification_leaderboards_pkey PRIMARY KEY (id);


--
-- Name: gamification_score_events gamification_score_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gamification_score_events
    ADD CONSTRAINT gamification_score_events_pkey PRIMARY KEY (id);


--
-- Name: gamification_scores gamification_scores_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gamification_scores
    ADD CONSTRAINT gamification_scores_pkey PRIMARY KEY (id);


--
-- Name: github_commits github_commits_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.github_commits
    ADD CONSTRAINT github_commits_pkey PRIMARY KEY (id);


--
-- Name: github_repos github_repos_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.github_repos
    ADD CONSTRAINT github_repos_pkey PRIMARY KEY (id);


--
-- Name: group_archived_messages group_archived_messages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.group_archived_messages
    ADD CONSTRAINT group_archived_messages_pkey PRIMARY KEY (id);


--
-- Name: group_associated_groups group_associated_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.group_associated_groups
    ADD CONSTRAINT group_associated_groups_pkey PRIMARY KEY (id);


--
-- Name: group_category_notification_defaults group_category_notification_defaults_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.group_category_notification_defaults
    ADD CONSTRAINT group_category_notification_defaults_pkey PRIMARY KEY (id);


--
-- Name: group_custom_fields group_custom_fields_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.group_custom_fields
    ADD CONSTRAINT group_custom_fields_pkey PRIMARY KEY (id);


--
-- Name: group_histories group_histories_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.group_histories
    ADD CONSTRAINT group_histories_pkey PRIMARY KEY (id);


--
-- Name: group_mentions group_mentions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.group_mentions
    ADD CONSTRAINT group_mentions_pkey PRIMARY KEY (id);


--
-- Name: group_requests group_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.group_requests
    ADD CONSTRAINT group_requests_pkey PRIMARY KEY (id);


--
-- Name: group_tag_notification_defaults group_tag_notification_defaults_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.group_tag_notification_defaults
    ADD CONSTRAINT group_tag_notification_defaults_pkey PRIMARY KEY (id);


--
-- Name: group_users group_users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.group_users
    ADD CONSTRAINT group_users_pkey PRIMARY KEY (id);


--
-- Name: groups groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.groups
    ADD CONSTRAINT groups_pkey PRIMARY KEY (id);


--
-- Name: ignored_users ignored_users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ignored_users
    ADD CONSTRAINT ignored_users_pkey PRIMARY KEY (id);


--
-- Name: incoming_chat_webhooks incoming_chat_webhooks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.incoming_chat_webhooks
    ADD CONSTRAINT incoming_chat_webhooks_pkey PRIMARY KEY (id);


--
-- Name: incoming_domains incoming_domains_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.incoming_domains
    ADD CONSTRAINT incoming_domains_pkey PRIMARY KEY (id);


--
-- Name: incoming_emails incoming_emails_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.incoming_emails
    ADD CONSTRAINT incoming_emails_pkey PRIMARY KEY (id);


--
-- Name: incoming_links incoming_links_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.incoming_links
    ADD CONSTRAINT incoming_links_pkey PRIMARY KEY (id);


--
-- Name: incoming_referers incoming_referers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.incoming_referers
    ADD CONSTRAINT incoming_referers_pkey PRIMARY KEY (id);


--
-- Name: inferred_concepts inferred_concepts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.inferred_concepts
    ADD CONSTRAINT inferred_concepts_pkey PRIMARY KEY (id);


--
-- Name: invited_groups invited_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invited_groups
    ADD CONSTRAINT invited_groups_pkey PRIMARY KEY (id);


--
-- Name: invited_users invited_users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invited_users
    ADD CONSTRAINT invited_users_pkey PRIMARY KEY (id);


--
-- Name: invites invites_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invites
    ADD CONSTRAINT invites_pkey PRIMARY KEY (id);


--
-- Name: javascript_caches javascript_caches_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.javascript_caches
    ADD CONSTRAINT javascript_caches_pkey PRIMARY KEY (id);


--
-- Name: linked_topics linked_topics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.linked_topics
    ADD CONSTRAINT linked_topics_pkey PRIMARY KEY (id);


--
-- Name: livestream_topic_chat_channels livestream_topic_chat_channels_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.livestream_topic_chat_channels
    ADD CONSTRAINT livestream_topic_chat_channels_pkey PRIMARY KEY (id);


--
-- Name: llm_credit_allocations llm_credit_allocations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.llm_credit_allocations
    ADD CONSTRAINT llm_credit_allocations_pkey PRIMARY KEY (id);


--
-- Name: llm_credit_daily_usages llm_credit_daily_usages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.llm_credit_daily_usages
    ADD CONSTRAINT llm_credit_daily_usages_pkey PRIMARY KEY (id);


--
-- Name: llm_feature_credit_costs llm_feature_credit_costs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.llm_feature_credit_costs
    ADD CONSTRAINT llm_feature_credit_costs_pkey PRIMARY KEY (id);


--
-- Name: llm_models llm_models_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.llm_models
    ADD CONSTRAINT llm_models_pkey PRIMARY KEY (id);


--
-- Name: llm_quota_usages llm_quota_usages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.llm_quota_usages
    ADD CONSTRAINT llm_quota_usages_pkey PRIMARY KEY (id);


--
-- Name: llm_quotas llm_quotas_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.llm_quotas
    ADD CONSTRAINT llm_quotas_pkey PRIMARY KEY (id);


--
-- Name: message_bus message_bus_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_bus
    ADD CONSTRAINT message_bus_pkey PRIMARY KEY (id);


--
-- Name: model_accuracies model_accuracies_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.model_accuracies
    ADD CONSTRAINT model_accuracies_pkey PRIMARY KEY (id);


--
-- Name: moved_posts moved_posts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.moved_posts
    ADD CONSTRAINT moved_posts_pkey PRIMARY KEY (id);


--
-- Name: muted_users muted_users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.muted_users
    ADD CONSTRAINT muted_users_pkey PRIMARY KEY (id);


--
-- Name: nested_topics nested_topics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.nested_topics
    ADD CONSTRAINT nested_topics_pkey PRIMARY KEY (id);


--
-- Name: nested_view_post_stats nested_view_post_stats_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.nested_view_post_stats
    ADD CONSTRAINT nested_view_post_stats_pkey PRIMARY KEY (id);


--
-- Name: notifications notifications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_pkey PRIMARY KEY (id);


--
-- Name: oauth2_user_infos oauth2_user_infos_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.oauth2_user_infos
    ADD CONSTRAINT oauth2_user_infos_pkey PRIMARY KEY (id);


--
-- Name: onceoff_logs onceoff_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.onceoff_logs
    ADD CONSTRAINT onceoff_logs_pkey PRIMARY KEY (id);


--
-- Name: optimized_images optimized_images_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.optimized_images
    ADD CONSTRAINT optimized_images_pkey PRIMARY KEY (id);


--
-- Name: optimized_videos optimized_videos_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.optimized_videos
    ADD CONSTRAINT optimized_videos_pkey PRIMARY KEY (id);


--
-- Name: permalinks permalinks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permalinks
    ADD CONSTRAINT permalinks_pkey PRIMARY KEY (id);


--
-- Name: plugin_store_rows plugin_store_rows_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.plugin_store_rows
    ADD CONSTRAINT plugin_store_rows_pkey PRIMARY KEY (id);


--
-- Name: policy_users policy_users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.policy_users
    ADD CONSTRAINT policy_users_pkey PRIMARY KEY (id);


--
-- Name: poll_options poll_options_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.poll_options
    ADD CONSTRAINT poll_options_pkey PRIMARY KEY (id);


--
-- Name: polls polls_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.polls
    ADD CONSTRAINT polls_pkey PRIMARY KEY (id);


--
-- Name: post_action_types post_action_types_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_action_types
    ADD CONSTRAINT post_action_types_pkey PRIMARY KEY (id);


--
-- Name: post_actions post_actions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_actions
    ADD CONSTRAINT post_actions_pkey PRIMARY KEY (id);


--
-- Name: post_custom_fields post_custom_fields_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_custom_fields
    ADD CONSTRAINT post_custom_fields_pkey PRIMARY KEY (id);


--
-- Name: post_custom_prompts post_custom_prompts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_custom_prompts
    ADD CONSTRAINT post_custom_prompts_pkey PRIMARY KEY (id);


--
-- Name: post_details post_details_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_details
    ADD CONSTRAINT post_details_pkey PRIMARY KEY (id);


--
-- Name: post_hotlinked_media post_hotlinked_media_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_hotlinked_media
    ADD CONSTRAINT post_hotlinked_media_pkey PRIMARY KEY (id);


--
-- Name: post_localizations post_localizations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_localizations
    ADD CONSTRAINT post_localizations_pkey PRIMARY KEY (id);


--
-- Name: post_policies post_policies_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_policies
    ADD CONSTRAINT post_policies_pkey PRIMARY KEY (id);


--
-- Name: post_policy_groups post_policy_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_policy_groups
    ADD CONSTRAINT post_policy_groups_pkey PRIMARY KEY (id);


--
-- Name: post_reply_keys post_reply_keys_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_reply_keys
    ADD CONSTRAINT post_reply_keys_pkey PRIMARY KEY (id);


--
-- Name: post_revisions post_revisions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_revisions
    ADD CONSTRAINT post_revisions_pkey PRIMARY KEY (id);


--
-- Name: post_stats post_stats_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_stats
    ADD CONSTRAINT post_stats_pkey PRIMARY KEY (id);


--
-- Name: post_voting_comment_custom_fields post_voting_comment_custom_fields_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_voting_comment_custom_fields
    ADD CONSTRAINT post_voting_comment_custom_fields_pkey PRIMARY KEY (id);


--
-- Name: post_voting_comments post_voting_comments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_voting_comments
    ADD CONSTRAINT post_voting_comments_pkey PRIMARY KEY (id);


--
-- Name: post_voting_votes post_voting_votes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_voting_votes
    ADD CONSTRAINT post_voting_votes_pkey PRIMARY KEY (id);


--
-- Name: posts posts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.posts
    ADD CONSTRAINT posts_pkey PRIMARY KEY (id);


--
-- Name: post_search_data posts_search_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.post_search_data
    ADD CONSTRAINT posts_search_pkey PRIMARY KEY (post_id);


--
-- Name: problem_check_trackers problem_check_trackers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.problem_check_trackers
    ADD CONSTRAINT problem_check_trackers_pkey PRIMARY KEY (id);


--
-- Name: published_pages published_pages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.published_pages
    ADD CONSTRAINT published_pages_pkey PRIMARY KEY (id);


--
-- Name: push_subscriptions push_subscriptions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.push_subscriptions
    ADD CONSTRAINT push_subscriptions_pkey PRIMARY KEY (id);


--
-- Name: quoted_posts quoted_posts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.quoted_posts
    ADD CONSTRAINT quoted_posts_pkey PRIMARY KEY (id);


--
-- Name: rag_document_fragments rag_document_fragments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rag_document_fragments
    ADD CONSTRAINT rag_document_fragments_pkey PRIMARY KEY (id);


--
-- Name: rag_document_sources rag_document_sources_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rag_document_sources
    ADD CONSTRAINT rag_document_sources_pkey PRIMARY KEY (id);


--
-- Name: redelivering_webhook_events redelivering_webhook_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.redelivering_webhook_events
    ADD CONSTRAINT redelivering_webhook_events_pkey PRIMARY KEY (id);


--
-- Name: remote_themes remote_themes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.remote_themes
    ADD CONSTRAINT remote_themes_pkey PRIMARY KEY (id);


--
-- Name: reviewable_claimed_topics reviewable_claimed_topics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviewable_claimed_topics
    ADD CONSTRAINT reviewable_claimed_topics_pkey PRIMARY KEY (id);


--
-- Name: reviewable_histories reviewable_histories_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviewable_histories
    ADD CONSTRAINT reviewable_histories_pkey PRIMARY KEY (id);


--
-- Name: reviewable_notes reviewable_notes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviewable_notes
    ADD CONSTRAINT reviewable_notes_pkey PRIMARY KEY (id);


--
-- Name: reviewable_scores reviewable_scores_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviewable_scores
    ADD CONSTRAINT reviewable_scores_pkey PRIMARY KEY (id);


--
-- Name: reviewables reviewables_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviewables
    ADD CONSTRAINT reviewables_pkey PRIMARY KEY (id);


--
-- Name: scheduler_stats scheduler_stats_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scheduler_stats
    ADD CONSTRAINT scheduler_stats_pkey PRIMARY KEY (id);


--
-- Name: schema_migration_details schema_migration_details_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migration_details
    ADD CONSTRAINT schema_migration_details_pkey PRIMARY KEY (id);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: screened_emails screened_emails_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.screened_emails
    ADD CONSTRAINT screened_emails_pkey PRIMARY KEY (id);


--
-- Name: screened_ip_addresses screened_ip_addresses_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.screened_ip_addresses
    ADD CONSTRAINT screened_ip_addresses_pkey PRIMARY KEY (id);


--
-- Name: screened_urls screened_urls_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.screened_urls
    ADD CONSTRAINT screened_urls_pkey PRIMARY KEY (id);


--
-- Name: search_logs search_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.search_logs
    ADD CONSTRAINT search_logs_pkey PRIMARY KEY (id);


--
-- Name: shared_ai_conversations shared_ai_conversations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.shared_ai_conversations
    ADD CONSTRAINT shared_ai_conversations_pkey PRIMARY KEY (id);


--
-- Name: shared_drafts shared_drafts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.shared_drafts
    ADD CONSTRAINT shared_drafts_pkey PRIMARY KEY (id);


--
-- Name: shelved_notifications shelved_notifications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.shelved_notifications
    ADD CONSTRAINT shelved_notifications_pkey PRIMARY KEY (id);


--
-- Name: sidebar_section_links sidebar_section_links_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sidebar_section_links
    ADD CONSTRAINT sidebar_section_links_pkey PRIMARY KEY (id);


--
-- Name: sidebar_section_localizations sidebar_section_localizations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sidebar_section_localizations
    ADD CONSTRAINT sidebar_section_localizations_pkey PRIMARY KEY (id);


--
-- Name: sidebar_sections sidebar_sections_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sidebar_sections
    ADD CONSTRAINT sidebar_sections_pkey PRIMARY KEY (id);


--
-- Name: sidebar_url_localizations sidebar_url_localizations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sidebar_url_localizations
    ADD CONSTRAINT sidebar_url_localizations_pkey PRIMARY KEY (id);


--
-- Name: sidebar_urls sidebar_urls_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sidebar_urls
    ADD CONSTRAINT sidebar_urls_pkey PRIMARY KEY (id);


--
-- Name: silenced_assignments silenced_assignments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.silenced_assignments
    ADD CONSTRAINT silenced_assignments_pkey PRIMARY KEY (id);


--
-- Name: single_sign_on_records single_sign_on_records_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.single_sign_on_records
    ADD CONSTRAINT single_sign_on_records_pkey PRIMARY KEY (id);


--
-- Name: site_setting_groups site_setting_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.site_setting_groups
    ADD CONSTRAINT site_setting_groups_pkey PRIMARY KEY (id);


--
-- Name: site_setting_localizations site_setting_localizations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.site_setting_localizations
    ADD CONSTRAINT site_setting_localizations_pkey PRIMARY KEY (id);


--
-- Name: site_settings site_settings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.site_settings
    ADD CONSTRAINT site_settings_pkey PRIMARY KEY (id);


--
-- Name: sitemaps sitemaps_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sitemaps
    ADD CONSTRAINT sitemaps_pkey PRIMARY KEY (id);


--
-- Name: skipped_email_logs skipped_email_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.skipped_email_logs
    ADD CONSTRAINT skipped_email_logs_pkey PRIMARY KEY (id);


--
-- Name: stylesheet_cache stylesheet_cache_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stylesheet_cache
    ADD CONSTRAINT stylesheet_cache_pkey PRIMARY KEY (id);


--
-- Name: summary_sections summary_sections_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.summary_sections
    ADD CONSTRAINT summary_sections_pkey PRIMARY KEY (id);


--
-- Name: tag_group_memberships tag_group_memberships_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_group_memberships
    ADD CONSTRAINT tag_group_memberships_pkey PRIMARY KEY (id);


--
-- Name: tag_group_permissions tag_group_permissions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_group_permissions
    ADD CONSTRAINT tag_group_permissions_pkey PRIMARY KEY (id);


--
-- Name: tag_groups tag_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_groups
    ADD CONSTRAINT tag_groups_pkey PRIMARY KEY (id);


--
-- Name: tag_localizations tag_localizations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_localizations
    ADD CONSTRAINT tag_localizations_pkey PRIMARY KEY (id);


--
-- Name: tag_search_data tag_search_data_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_search_data
    ADD CONSTRAINT tag_search_data_pkey PRIMARY KEY (tag_id);


--
-- Name: tag_users tag_users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_users
    ADD CONSTRAINT tag_users_pkey PRIMARY KEY (id);


--
-- Name: tags tags_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tags
    ADD CONSTRAINT tags_pkey PRIMARY KEY (id);


--
-- Name: theme_fields theme_fields_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.theme_fields
    ADD CONSTRAINT theme_fields_pkey PRIMARY KEY (id);


--
-- Name: theme_modifier_sets theme_modifier_sets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.theme_modifier_sets
    ADD CONSTRAINT theme_modifier_sets_pkey PRIMARY KEY (id);


--
-- Name: theme_settings_migrations theme_settings_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.theme_settings_migrations
    ADD CONSTRAINT theme_settings_migrations_pkey PRIMARY KEY (id);


--
-- Name: theme_settings theme_settings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.theme_settings
    ADD CONSTRAINT theme_settings_pkey PRIMARY KEY (id);


--
-- Name: theme_site_settings theme_site_settings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.theme_site_settings
    ADD CONSTRAINT theme_site_settings_pkey PRIMARY KEY (id);


--
-- Name: theme_svg_sprites theme_svg_sprites_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.theme_svg_sprites
    ADD CONSTRAINT theme_svg_sprites_pkey PRIMARY KEY (id);


--
-- Name: theme_translation_overrides theme_translation_overrides_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.theme_translation_overrides
    ADD CONSTRAINT theme_translation_overrides_pkey PRIMARY KEY (id);


--
-- Name: themes themes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.themes
    ADD CONSTRAINT themes_pkey PRIMARY KEY (id);


--
-- Name: top_topics top_topics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.top_topics
    ADD CONSTRAINT top_topics_pkey PRIMARY KEY (id);


--
-- Name: topic_allowed_groups topic_allowed_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_allowed_groups
    ADD CONSTRAINT topic_allowed_groups_pkey PRIMARY KEY (id);


--
-- Name: topic_allowed_users topic_allowed_users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_allowed_users
    ADD CONSTRAINT topic_allowed_users_pkey PRIMARY KEY (id);


--
-- Name: topic_custom_fields topic_custom_fields_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_custom_fields
    ADD CONSTRAINT topic_custom_fields_pkey PRIMARY KEY (id);


--
-- Name: topic_embeds topic_embeds_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_embeds
    ADD CONSTRAINT topic_embeds_pkey PRIMARY KEY (id);


--
-- Name: topic_groups topic_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_groups
    ADD CONSTRAINT topic_groups_pkey PRIMARY KEY (id);


--
-- Name: topic_hot_scores topic_hot_scores_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_hot_scores
    ADD CONSTRAINT topic_hot_scores_pkey PRIMARY KEY (id);


--
-- Name: topic_invites topic_invites_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_invites
    ADD CONSTRAINT topic_invites_pkey PRIMARY KEY (id);


--
-- Name: topic_link_clicks topic_link_clicks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_link_clicks
    ADD CONSTRAINT topic_link_clicks_pkey PRIMARY KEY (id);


--
-- Name: topic_links topic_links_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_links
    ADD CONSTRAINT topic_links_pkey PRIMARY KEY (id);


--
-- Name: topic_localizations topic_localizations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_localizations
    ADD CONSTRAINT topic_localizations_pkey PRIMARY KEY (id);


--
-- Name: topic_search_data topic_search_data_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_search_data
    ADD CONSTRAINT topic_search_data_pkey PRIMARY KEY (topic_id);


--
-- Name: topic_tags topic_tags_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_tags
    ADD CONSTRAINT topic_tags_pkey PRIMARY KEY (id);


--
-- Name: topic_thumbnails topic_thumbnails_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_thumbnails
    ADD CONSTRAINT topic_thumbnails_pkey PRIMARY KEY (id);


--
-- Name: topic_timers topic_timers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_timers
    ADD CONSTRAINT topic_timers_pkey PRIMARY KEY (id);


--
-- Name: topic_users topic_users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_users
    ADD CONSTRAINT topic_users_pkey PRIMARY KEY (id);


--
-- Name: topic_view_stats topic_view_stats_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_view_stats
    ADD CONSTRAINT topic_view_stats_pkey PRIMARY KEY (id);


--
-- Name: topic_voting_category_settings topic_voting_category_settings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_voting_category_settings
    ADD CONSTRAINT topic_voting_category_settings_pkey PRIMARY KEY (id);


--
-- Name: topic_voting_topic_vote_count topic_voting_topic_vote_count_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_voting_topic_vote_count
    ADD CONSTRAINT topic_voting_topic_vote_count_pkey PRIMARY KEY (id);


--
-- Name: topic_voting_votes topic_voting_votes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topic_voting_votes
    ADD CONSTRAINT topic_voting_votes_pkey PRIMARY KEY (id);


--
-- Name: topics topics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.topics
    ADD CONSTRAINT topics_pkey PRIMARY KEY (id);


--
-- Name: translation_overrides translation_overrides_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.translation_overrides
    ADD CONSTRAINT translation_overrides_pkey PRIMARY KEY (id);


--
-- Name: upcoming_change_events upcoming_change_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.upcoming_change_events
    ADD CONSTRAINT upcoming_change_events_pkey PRIMARY KEY (id);


--
-- Name: upload_references upload_references_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.upload_references
    ADD CONSTRAINT upload_references_pkey PRIMARY KEY (id);


--
-- Name: uploads uploads_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.uploads
    ADD CONSTRAINT uploads_pkey PRIMARY KEY (id);


--
-- Name: user_actions user_actions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_actions
    ADD CONSTRAINT user_actions_pkey PRIMARY KEY (id);


--
-- Name: user_api_key_client_scopes user_api_key_client_scopes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_api_key_client_scopes
    ADD CONSTRAINT user_api_key_client_scopes_pkey PRIMARY KEY (id);


--
-- Name: user_api_key_clients user_api_key_clients_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_api_key_clients
    ADD CONSTRAINT user_api_key_clients_pkey PRIMARY KEY (id);


--
-- Name: user_api_key_scopes user_api_key_scopes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_api_key_scopes
    ADD CONSTRAINT user_api_key_scopes_pkey PRIMARY KEY (id);


--
-- Name: user_api_keys user_api_keys_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_api_keys
    ADD CONSTRAINT user_api_keys_pkey PRIMARY KEY (id);


--
-- Name: user_archived_messages user_archived_messages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_archived_messages
    ADD CONSTRAINT user_archived_messages_pkey PRIMARY KEY (id);


--
-- Name: user_associated_accounts user_associated_accounts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_associated_accounts
    ADD CONSTRAINT user_associated_accounts_pkey PRIMARY KEY (id);


--
-- Name: user_associated_groups user_associated_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_associated_groups
    ADD CONSTRAINT user_associated_groups_pkey PRIMARY KEY (id);


--
-- Name: user_auth_token_logs user_auth_token_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_auth_token_logs
    ADD CONSTRAINT user_auth_token_logs_pkey PRIMARY KEY (id);


--
-- Name: user_auth_tokens user_auth_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_auth_tokens
    ADD CONSTRAINT user_auth_tokens_pkey PRIMARY KEY (id);


--
-- Name: user_avatars user_avatars_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_avatars
    ADD CONSTRAINT user_avatars_pkey PRIMARY KEY (id);


--
-- Name: user_badges user_badges_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_badges
    ADD CONSTRAINT user_badges_pkey PRIMARY KEY (id);


--
-- Name: user_chat_channel_memberships user_chat_channel_memberships_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_chat_channel_memberships
    ADD CONSTRAINT user_chat_channel_memberships_pkey PRIMARY KEY (id);


--
-- Name: user_chat_thread_memberships user_chat_thread_memberships_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_chat_thread_memberships
    ADD CONSTRAINT user_chat_thread_memberships_pkey PRIMARY KEY (id);


--
-- Name: user_custom_fields user_custom_fields_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_custom_fields
    ADD CONSTRAINT user_custom_fields_pkey PRIMARY KEY (id);


--
-- Name: user_emails user_emails_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_emails
    ADD CONSTRAINT user_emails_pkey PRIMARY KEY (id);


--
-- Name: user_exports user_exports_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_exports
    ADD CONSTRAINT user_exports_pkey PRIMARY KEY (id);


--
-- Name: user_field_options user_field_options_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_field_options
    ADD CONSTRAINT user_field_options_pkey PRIMARY KEY (id);


--
-- Name: user_fields user_fields_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_fields
    ADD CONSTRAINT user_fields_pkey PRIMARY KEY (id);


--
-- Name: user_histories user_histories_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_histories
    ADD CONSTRAINT user_histories_pkey PRIMARY KEY (id);


--
-- Name: user_ip_address_histories user_ip_address_histories_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_ip_address_histories
    ADD CONSTRAINT user_ip_address_histories_pkey PRIMARY KEY (id);


--
-- Name: user_notification_schedules user_notification_schedules_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_notification_schedules
    ADD CONSTRAINT user_notification_schedules_pkey PRIMARY KEY (id);


--
-- Name: user_open_ids user_open_ids_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_open_ids
    ADD CONSTRAINT user_open_ids_pkey PRIMARY KEY (id);


--
-- Name: user_passwords user_passwords_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_passwords
    ADD CONSTRAINT user_passwords_pkey PRIMARY KEY (id);


--
-- Name: user_profile_views user_profile_views_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_profile_views
    ADD CONSTRAINT user_profile_views_pkey PRIMARY KEY (id);


--
-- Name: user_profiles user_profiles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_profiles
    ADD CONSTRAINT user_profiles_pkey PRIMARY KEY (user_id);


--
-- Name: user_required_fields_versions user_required_fields_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_required_fields_versions
    ADD CONSTRAINT user_required_fields_versions_pkey PRIMARY KEY (id);


--
-- Name: user_second_factors user_second_factors_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_second_factors
    ADD CONSTRAINT user_second_factors_pkey PRIMARY KEY (id);


--
-- Name: user_security_keys user_security_keys_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_security_keys
    ADD CONSTRAINT user_security_keys_pkey PRIMARY KEY (id);


--
-- Name: user_stats user_stats_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_stats
    ADD CONSTRAINT user_stats_pkey PRIMARY KEY (user_id);


--
-- Name: user_statuses user_statuses_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_statuses
    ADD CONSTRAINT user_statuses_pkey PRIMARY KEY (user_id);


--
-- Name: user_uploads user_uploads_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_uploads
    ADD CONSTRAINT user_uploads_pkey PRIMARY KEY (id);


--
-- Name: user_visit_daily_rollups user_visit_daily_rollups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_visit_daily_rollups
    ADD CONSTRAINT user_visit_daily_rollups_pkey PRIMARY KEY (id);


--
-- Name: user_visits user_visits_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_visits
    ADD CONSTRAINT user_visits_pkey PRIMARY KEY (id);


--
-- Name: user_warnings user_warnings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_warnings
    ADD CONSTRAINT user_warnings_pkey PRIMARY KEY (id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: user_search_data users_search_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_search_data
    ADD CONSTRAINT users_search_pkey PRIMARY KEY (user_id);


--
-- Name: voice_co_presences voice_co_presences_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.voice_co_presences
    ADD CONSTRAINT voice_co_presences_pkey PRIMARY KEY (id);


--
-- Name: voice_invites voice_invites_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.voice_invites
    ADD CONSTRAINT voice_invites_pkey PRIMARY KEY (id);


--
-- Name: voice_recordings voice_recordings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.voice_recordings
    ADD CONSTRAINT voice_recordings_pkey PRIMARY KEY (id);


--
-- Name: voice_room_memberships voice_room_memberships_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.voice_room_memberships
    ADD CONSTRAINT voice_room_memberships_pkey PRIMARY KEY (id);


--
-- Name: voice_rooms voice_rooms_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.voice_rooms
    ADD CONSTRAINT voice_rooms_pkey PRIMARY KEY (id);


--
-- Name: voice_sessions voice_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.voice_sessions
    ADD CONSTRAINT voice_sessions_pkey PRIMARY KEY (id);


--
-- Name: watched_word_groups watched_word_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.watched_word_groups
    ADD CONSTRAINT watched_word_groups_pkey PRIMARY KEY (id);


--
-- Name: watched_words watched_words_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.watched_words
    ADD CONSTRAINT watched_words_pkey PRIMARY KEY (id);


--
-- Name: web_crawler_requests web_crawler_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.web_crawler_requests
    ADD CONSTRAINT web_crawler_requests_pkey PRIMARY KEY (id);


--
-- Name: web_hook_event_types web_hook_event_types_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.web_hook_event_types
    ADD CONSTRAINT web_hook_event_types_pkey PRIMARY KEY (id);


--
-- Name: web_hook_events_daily_aggregates web_hook_events_daily_aggregates_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.web_hook_events_daily_aggregates
    ADD CONSTRAINT web_hook_events_daily_aggregates_pkey PRIMARY KEY (id);


--
-- Name: web_hook_events web_hook_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.web_hook_events
    ADD CONSTRAINT web_hook_events_pkey PRIMARY KEY (id);


--
-- Name: web_hooks web_hooks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.web_hooks
    ADD CONSTRAINT web_hooks_pkey PRIMARY KEY (id);


--
-- Name: associated_accounts_provider_uid; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX associated_accounts_provider_uid ON public.user_associated_accounts USING btree (provider_name, provider_uid);


--
-- Name: associated_accounts_provider_user; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX associated_accounts_provider_user ON public.user_associated_accounts USING btree (provider_name, user_id);


--
-- Name: associated_groups_provider_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX associated_groups_provider_id ON public.associated_groups USING btree (provider_name, provider_id);


--
-- Name: by_link; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX by_link ON public.topic_link_clicks USING btree (topic_link_id);


--
-- Name: cat_featured_threads; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX cat_featured_threads ON public.category_featured_topics USING btree (category_id, topic_id);


--
-- Name: chat_message_reactions_index; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX chat_message_reactions_index ON public.chat_message_reactions USING btree (chat_message_id, user_id, emoji);


--
-- Name: chat_webhook_events_index; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX chat_webhook_events_index ON public.chat_webhook_events USING btree (chat_message_id, incoming_chat_webhook_id);


--
-- Name: direct_message_users_index; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX direct_message_users_index ON public.direct_message_users USING btree (direct_message_channel_id, user_id);


--
-- Name: directory_column_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX directory_column_index ON public.directory_columns USING btree (enabled, "position", user_field_id);


--
-- Name: discourse_post_event_invitees_post_id_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX discourse_post_event_invitees_post_id_user_id_idx ON public.discourse_post_event_invitees USING btree (post_id, user_id);


--
-- Name: idx_access_control_lists_allowed_group_ids; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_access_control_lists_allowed_group_ids ON public.access_control_lists USING gin (allowed_group_ids);


--
-- Name: idx_access_control_lists_allowed_user_ids; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_access_control_lists_allowed_user_ids ON public.access_control_lists USING gin (allowed_user_ids);


--
-- Name: idx_ai_api_audit_logs_failed_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_api_audit_logs_failed_id ON public.ai_api_audit_logs USING btree (id) WHERE (((response_status IS NOT NULL) AND ((response_status < 200) OR (response_status > 299))) OR ((response_status IS NULL) AND (COALESCE(response_tokens, 0) <= 0)));


--
-- Name: idx_ai_api_audit_logs_feature_name_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_api_audit_logs_feature_name_id ON public.ai_api_audit_logs USING btree (feature_name, id) WHERE (feature_name IS NOT NULL);


--
-- Name: idx_ai_api_audit_logs_payload_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_api_audit_logs_payload_id ON public.ai_api_audit_logs USING btree (id) WHERE ((raw_request_payload IS NOT NULL) OR (raw_response_payload IS NOT NULL));


--
-- Name: idx_ai_api_audit_logs_retried_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_api_audit_logs_retried_id ON public.ai_api_audit_logs USING btree (id) WHERE (request_attempts IS NOT NULL);


--
-- Name: idx_ai_api_audit_logs_user_id_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_api_audit_logs_user_id_id ON public.ai_api_audit_logs USING btree (user_id, id);


--
-- Name: idx_ai_bot_conversation_stars_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_bot_conversation_stars_topic_id ON public.discourse_ai_ai_bot_conversation_stars USING btree (topic_id);


--
-- Name: idx_ai_bot_conversation_stars_user_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_bot_conversation_stars_user_created ON public.discourse_ai_ai_bot_conversation_stars USING btree (user_id, created_at);


--
-- Name: idx_ai_bot_conversation_stars_user_topic; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_ai_bot_conversation_stars_user_topic ON public.discourse_ai_ai_bot_conversation_stars USING btree (user_id, topic_id);


--
-- Name: idx_ai_post_image_captions_lookup; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_ai_post_image_captions_lookup ON public.ai_post_image_captions USING btree (post_id, locale, base62_sha1);


--
-- Name: idx_ai_post_image_captions_reuse; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_post_image_captions_reuse ON public.ai_post_image_captions USING btree (base62_sha1, locale);


--
-- Name: idx_ai_summaries_on_target_type_and_locale; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_ai_summaries_on_target_type_and_locale ON public.ai_summaries USING btree (target_id, target_type, summary_type, locale) NULLS NOT DISTINCT;


--
-- Name: idx_bookmarks_user_polymorphic_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_bookmarks_user_polymorphic_unique ON public.bookmarks USING btree (user_id, bookmarkable_type, bookmarkable_id);


--
-- Name: idx_bpcd_rollups_date_country_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_bpcd_rollups_date_country_unique ON public.browser_pageview_country_daily_rollups USING btree (date, country_code) NULLS NOT DISTINCT;


--
-- Name: idx_bpcrawler_rollups_date_logged_in_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_bpcrawler_rollups_date_logged_in_unique ON public.browser_pageview_crawler_daily_rollups USING btree (date, logged_in);


--
-- Name: idx_bpe_beacon_created_at_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_bpe_beacon_created_at_id ON public.browser_pageview_events USING btree (created_at DESC, id DESC) WHERE (source = 2);


--
-- Name: idx_bpe_browser_backfill; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_bpe_browser_backfill ON public.browser_pageview_events USING btree (source, created_at DESC, id DESC) WHERE (browser IS NULL);


--
-- Name: idx_bpe_created_at_country_code; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_bpe_created_at_country_code ON public.browser_pageview_events USING btree (created_at, country_code);


--
-- Name: idx_bpe_created_at_normalized_referrer; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_bpe_created_at_normalized_referrer ON public.browser_pageview_events USING btree (created_at, normalized_referrer);


--
-- Name: idx_bpe_ip_ua_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_bpe_ip_ua_created_at ON public.browser_pageview_events USING btree (ip_address, user_agent, created_at);


--
-- Name: idx_bpe_normalized_referrer_version; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_bpe_normalized_referrer_version ON public.browser_pageview_events USING btree (normalized_referrer_version) WHERE (referrer IS NOT NULL);


--
-- Name: idx_bpe_normalized_url_version; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_bpe_normalized_url_version ON public.browser_pageview_events USING btree (normalized_url_version);


--
-- Name: idx_bpe_session_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_bpe_session_created_at ON public.browser_pageview_events USING btree (session_id, created_at);


--
-- Name: idx_bpeu_daily_rollups_date_url_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_bpeu_daily_rollups_date_url_unique ON public.browser_pageview_entry_url_daily_rollups USING btree (date, entry_url);


--
-- Name: idx_bprd_rollups_date_referrer_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_bprd_rollups_date_referrer_unique ON public.browser_pageview_referrer_daily_rollups USING btree (date, normalized_referrer) NULLS NOT DISTINCT;


--
-- Name: idx_bpse_rollups_date_logged_in_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_bpse_rollups_date_logged_in_unique ON public.browser_pageview_session_engagement_daily_rollups USING btree (date, logged_in);


--
-- Name: idx_category_posting_review_groups_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_category_posting_review_groups_unique ON public.category_posting_review_groups USING btree (category_id, group_id, post_type);


--
-- Name: idx_category_required_tag_groups; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_category_required_tag_groups ON public.category_required_tag_groups USING btree (category_id, tag_group_id);


--
-- Name: idx_category_tag_groups_ix1; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_category_tag_groups_ix1 ON public.category_tag_groups USING btree (category_id, tag_group_id);


--
-- Name: idx_category_tags_ix1; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_category_tags_ix1 ON public.category_tags USING btree (category_id, tag_id);


--
-- Name: idx_category_tags_ix2; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_category_tags_ix2 ON public.category_tags USING btree (tag_id, category_id);


--
-- Name: idx_category_users_category_id_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_category_users_category_id_user_id ON public.category_users USING btree (category_id, user_id);


--
-- Name: idx_category_users_user_id_category_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_category_users_user_id_category_id ON public.category_users USING btree (user_id, category_id);


--
-- Name: idx_chat_messages_by_created_at_not_deleted; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_chat_messages_by_created_at_not_deleted ON public.chat_messages USING btree (created_at) WHERE (deleted_at IS NULL);


--
-- Name: idx_chat_messages_by_thread_id_not_deleted; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_chat_messages_by_thread_id_not_deleted ON public.chat_messages USING btree (thread_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_chat_messages_thread_id_id_user_id_not_deleted; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_chat_messages_thread_id_id_user_id_not_deleted ON public.chat_messages USING btree (thread_id, id) INCLUDE (user_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_chat_pinned_messages_channel_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_chat_pinned_messages_channel_created ON public.chat_pinned_messages USING btree (chat_channel_id, created_at DESC);


--
-- Name: idx_discourse_automation_user_global_notices; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_discourse_automation_user_global_notices ON public.discourse_automation_user_global_notices USING btree (user_id, identifier);


--
-- Name: idx_discourse_calendar_post_event_dates_event_id_starts_at_uniq; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_discourse_calendar_post_event_dates_event_id_starts_at_uniq ON public.discourse_calendar_post_event_dates USING btree (event_id, starts_at);


--
-- Name: idx_dwf_ai_sessions_on_status_updated_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_ai_sessions_on_status_updated_at ON public.discourse_workflows_ai_authoring_sessions USING btree (status, updated_at);


--
-- Name: idx_dwf_ai_sessions_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_ai_sessions_on_user_id ON public.discourse_workflows_ai_authoring_sessions USING btree (user_id);


--
-- Name: idx_dwf_ai_sessions_on_workflow_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_ai_sessions_on_workflow_id ON public.discourse_workflows_ai_authoring_sessions USING btree (workflow_id);


--
-- Name: idx_dwf_call_runs_on_child_execution_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_dwf_call_runs_on_child_execution_id ON public.discourse_workflows_workflow_call_runs USING btree (child_execution_id) WHERE (child_execution_id IS NOT NULL);


--
-- Name: idx_dwf_call_runs_on_parent_execution_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_call_runs_on_parent_execution_id ON public.discourse_workflows_workflow_call_runs USING btree (parent_execution_id);


--
-- Name: idx_dwf_credentials_on_created_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_credentials_on_created_by_id ON public.discourse_workflows_credentials USING btree (created_by_id);


--
-- Name: idx_dwf_credentials_on_credential_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_credentials_on_credential_type ON public.discourse_workflows_credentials USING btree (credential_type);


--
-- Name: idx_dwf_credentials_on_name_credential_type; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_dwf_credentials_on_name_credential_type ON public.discourse_workflows_credentials USING btree (name, credential_type);


--
-- Name: idx_dwf_credentials_on_updated_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_credentials_on_updated_by_id ON public.discourse_workflows_credentials USING btree (updated_by_id);


--
-- Name: idx_dwf_data_tables_on_created_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_data_tables_on_created_by_id ON public.discourse_workflows_data_tables USING btree (created_by_id);


--
-- Name: idx_dwf_data_tables_on_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_dwf_data_tables_on_name ON public.discourse_workflows_data_tables USING btree (name);


--
-- Name: idx_dwf_data_tables_on_updated_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_data_tables_on_updated_by_id ON public.discourse_workflows_data_tables USING btree (updated_by_id);


--
-- Name: idx_dwf_deps_on_type_key; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_deps_on_type_key ON public.discourse_workflows_workflow_dependencies USING btree (dependency_type, dependency_key);


--
-- Name: idx_dwf_deps_on_workflow_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_deps_on_workflow_id ON public.discourse_workflows_workflow_dependencies USING btree (workflow_id);


--
-- Name: idx_dwf_deps_on_workflow_version_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_deps_on_workflow_version_id ON public.discourse_workflows_workflow_dependencies USING btree (workflow_version_id);


--
-- Name: idx_dwf_execution_data_on_execution_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_dwf_execution_data_on_execution_id ON public.discourse_workflows_execution_data USING btree (execution_id);


--
-- Name: idx_dwf_execution_stats_on_workflow_id_and_date; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_dwf_execution_stats_on_workflow_id_and_date ON public.discourse_workflows_execution_stats USING btree (workflow_id, date);


--
-- Name: idx_dwf_executions_on_resume_token; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_executions_on_resume_token ON public.discourse_workflows_executions USING btree (resume_token) WHERE (resume_token IS NOT NULL);


--
-- Name: idx_dwf_executions_on_retention; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_executions_on_retention ON public.discourse_workflows_executions USING btree (created_at) WHERE (status = ANY (ARRAY[2, 3, 5, 6]));


--
-- Name: idx_dwf_executions_on_status_waiting_until; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_executions_on_status_waiting_until ON public.discourse_workflows_executions USING btree (status, waiting_until);


--
-- Name: idx_dwf_executions_on_waiting_until; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_executions_on_waiting_until ON public.discourse_workflows_executions USING btree (waiting_until) WHERE ((waiting_until IS NOT NULL) AND (status = 4));


--
-- Name: idx_dwf_executions_on_workflow_created_at_id_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_executions_on_workflow_created_at_id_desc ON public.discourse_workflows_executions USING btree (workflow_id, created_at DESC, id DESC);


--
-- Name: idx_dwf_executions_on_workflow_version_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_executions_on_workflow_version_id ON public.discourse_workflows_executions USING btree (workflow_version_id);


--
-- Name: idx_dwf_publish_history_on_workflow_created_at_id_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_publish_history_on_workflow_created_at_id_desc ON public.discourse_workflows_workflow_publish_history USING btree (workflow_id, created_at DESC, id DESC);


--
-- Name: idx_dwf_tags_on_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_dwf_tags_on_name ON public.discourse_workflows_tags USING btree (name);


--
-- Name: idx_dwf_variables_on_created_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_variables_on_created_by_id ON public.discourse_workflows_variables USING btree (created_by_id);


--
-- Name: idx_dwf_variables_on_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_dwf_variables_on_key ON public.discourse_workflows_variables USING btree (key);


--
-- Name: idx_dwf_versions_on_workflow_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_versions_on_workflow_created_at ON public.discourse_workflows_workflow_versions USING btree (workflow_id, created_at DESC);


--
-- Name: idx_dwf_versions_on_workflow_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_versions_on_workflow_id ON public.discourse_workflows_workflow_versions USING btree (workflow_id);


--
-- Name: idx_dwf_versions_on_workflow_version_number; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_dwf_versions_on_workflow_version_number ON public.discourse_workflows_workflow_versions USING btree (workflow_id, version_number);


--
-- Name: idx_dwf_webhooks_on_expires_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_webhooks_on_expires_at ON public.discourse_workflows_webhooks USING btree (expires_at) WHERE (expires_at IS NOT NULL);


--
-- Name: idx_dwf_webhooks_on_method_path_test; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_dwf_webhooks_on_method_path_test ON public.discourse_workflows_webhooks USING btree (http_method, webhook_path, test_webhook);


--
-- Name: idx_dwf_webhooks_on_webhook_id_method_test; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_webhooks_on_webhook_id_method_test ON public.discourse_workflows_webhooks USING btree (webhook_id, http_method, test_webhook) WHERE (webhook_id IS NOT NULL);


--
-- Name: idx_dwf_webhooks_on_workflow_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_webhooks_on_workflow_id ON public.discourse_workflows_webhooks USING btree (workflow_id);


--
-- Name: idx_dwf_webhooks_on_workflow_version_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_webhooks_on_workflow_version_id ON public.discourse_workflows_webhooks USING btree (workflow_version_id);


--
-- Name: idx_dwf_workflow_tags_on_tag_workflow; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_dwf_workflow_tags_on_tag_workflow ON public.discourse_workflows_workflow_tags USING btree (workflow_tag_id, workflow_id);


--
-- Name: idx_dwf_workflow_tags_on_workflow_tag; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_dwf_workflow_tags_on_workflow_tag ON public.discourse_workflows_workflow_tags USING btree (workflow_id, workflow_tag_id);


--
-- Name: idx_dwf_workflows_on_active_version_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_workflows_on_active_version_id ON public.discourse_workflows_workflows USING btree (active_version_id);


--
-- Name: idx_dwf_workflows_on_created_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_workflows_on_created_by_id ON public.discourse_workflows_workflows USING btree (created_by_id);


--
-- Name: idx_dwf_workflows_on_error_workflow_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_workflows_on_error_workflow_id ON public.discourse_workflows_workflows USING btree (error_workflow_id);


--
-- Name: idx_dwf_workflows_on_updated_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dwf_workflows_on_updated_by_id ON public.discourse_workflows_workflows USING btree (updated_by_id);


--
-- Name: idx_dwf_workflows_on_version_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_dwf_workflows_on_version_id ON public.discourse_workflows_workflows USING btree (version_id);


--
-- Name: idx_email_change_requests_on_requested_by; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_email_change_requests_on_requested_by ON public.email_change_requests USING btree (requested_by_user_id);


--
-- Name: idx_group_category_notification_defaults_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_group_category_notification_defaults_unique ON public.group_category_notification_defaults USING btree (group_id, category_id);


--
-- Name: idx_group_tag_notification_defaults_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_group_tag_notification_defaults_unique ON public.group_tag_notification_defaults USING btree (group_id, tag_id);


--
-- Name: idx_kanban_boards_category_ids; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kanban_boards_category_ids ON public.discourse_kanban_boards USING gin (category_ids);


--
-- Name: idx_kanban_boards_tag_ids; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kanban_boards_tag_ids ON public.discourse_kanban_boards USING gin (tag_ids);


--
-- Name: idx_kanban_cards_assigned_to; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kanban_cards_assigned_to ON public.discourse_kanban_cards USING btree (assigned_to_type, assigned_to_id);


--
-- Name: idx_kanban_cards_board_column_position; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kanban_cards_board_column_position ON public.discourse_kanban_cards USING btree (board_id, column_id, "position");


--
-- Name: idx_kanban_cards_board_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kanban_cards_board_id ON public.discourse_kanban_cards USING btree (board_id);


--
-- Name: idx_kanban_cards_column_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kanban_cards_column_id ON public.discourse_kanban_cards USING btree (column_id);


--
-- Name: idx_kanban_cards_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kanban_cards_topic_id ON public.discourse_kanban_cards USING btree (topic_id);


--
-- Name: idx_kanban_cards_unique_topic_per_column; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_kanban_cards_unique_topic_per_column ON public.discourse_kanban_cards USING btree (board_id, column_id, topic_id) WHERE ((topic_id IS NOT NULL) AND (column_id IS NOT NULL));


--
-- Name: idx_kanban_columns_board_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kanban_columns_board_id ON public.discourse_kanban_columns USING btree (board_id);


--
-- Name: idx_kanban_columns_board_position; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kanban_columns_board_position ON public.discourse_kanban_columns USING btree (board_id, "position");


--
-- Name: idx_kanban_columns_tag_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kanban_columns_tag_id ON public.discourse_kanban_columns USING btree (tag_id);


--
-- Name: idx_leaderboard_scores_lb_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_leaderboard_scores_lb_date ON public.gamification_leaderboard_scores USING btree (leaderboard_id, date);


--
-- Name: idx_leaderboard_scores_lb_user_date; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_leaderboard_scores_lb_user_date ON public.gamification_leaderboard_scores USING btree (leaderboard_id, user_id, date);


--
-- Name: idx_notifications_speedup_unread_count; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_speedup_unread_count ON public.notifications USING btree (user_id, notification_type) WHERE (NOT read);


--
-- Name: idx_on_llm_model_id_feature_name_2b0b794b27; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_on_llm_model_id_feature_name_2b0b794b27 ON public.llm_feature_credit_costs USING btree (llm_model_id, feature_name);


--
-- Name: idx_on_sidebar_section_id_locale_271bd8ee1c; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_on_sidebar_section_id_locale_271bd8ee1c ON public.sidebar_section_localizations USING btree (sidebar_section_id, locale);


--
-- Name: idx_on_target_type_target_id_permission_f472902150; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_on_target_type_target_id_permission_f472902150 ON public.access_control_lists USING btree (target_type, target_id, permission);


--
-- Name: idx_post_voting_comment_custom_fields; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_post_voting_comment_custom_fields ON public.post_voting_comment_custom_fields USING btree (post_voting_comment_id, name);


--
-- Name: idx_posts_created_at_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_posts_created_at_topic_id ON public.posts USING btree (created_at, topic_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_posts_deleted_posts; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_posts_deleted_posts ON public.posts USING btree (topic_id, post_number) WHERE (deleted_at IS NOT NULL);


--
-- Name: idx_posts_user_id_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_posts_user_id_deleted_at ON public.posts USING btree (user_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_rag_document_sources_target_url; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_rag_document_sources_target_url ON public.rag_document_sources USING btree (target_type, target_id, url_digest);


--
-- Name: idx_reviewables_score_desc_created_at_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_reviewables_score_desc_created_at_desc ON public.reviewables USING btree (score DESC, created_at DESC);


--
-- Name: idx_rss_polling_poll_attempts_on_feed_created_id_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_rss_polling_poll_attempts_on_feed_created_id_desc ON public.discourse_rss_polling_poll_attempts USING btree (rss_feed_id, created_at DESC, id DESC);


--
-- Name: idx_search_category; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_search_category ON public.category_search_data USING gin (search_data);


--
-- Name: idx_search_chat_message; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_search_chat_message ON public.chat_message_search_data USING gin (search_data);


--
-- Name: idx_search_post; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_search_post ON public.post_search_data USING gin (search_data);


--
-- Name: idx_search_tag; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_search_tag ON public.tag_search_data USING gin (search_data);


--
-- Name: idx_search_topic; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_search_topic ON public.topic_search_data USING gin (search_data);


--
-- Name: idx_search_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_search_user ON public.user_search_data USING gin (search_data);


--
-- Name: idx_shared_ai_conversations_user_target; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_shared_ai_conversations_user_target ON public.shared_ai_conversations USING btree (user_id, target_id, target_type);


--
-- Name: idx_sidebar_section_links_on_sidebar_section_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_sidebar_section_links_on_sidebar_section_id ON public.sidebar_section_links USING btree (sidebar_section_id, user_id, "position");


--
-- Name: idx_timerable_id_public_type_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_timerable_id_public_type_deleted_at ON public.topic_timers USING btree (timerable_id) WHERE ((public_type = true) AND (deleted_at IS NULL) AND ((type)::text = 'TopicTimer'::text));


--
-- Name: idx_topic_custom_fields_accepted_answer; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_topic_custom_fields_accepted_answer ON public.topic_custom_fields USING btree (topic_id) WHERE ((name)::text = 'accepted_answer_post_id'::text);


--
-- Name: idx_topic_custom_fields_auto_responder_triggered_ids_partial; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_topic_custom_fields_auto_responder_triggered_ids_partial ON public.topic_custom_fields USING btree (topic_id, value) WHERE ((name)::text = 'auto_responder_triggered_ids'::text);


--
-- Name: idx_topic_custom_fields_topic_post_event_all_day; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_topic_custom_fields_topic_post_event_all_day ON public.topic_custom_fields USING btree (name, topic_id) WHERE ((name)::text = 'TopicEventAllDay'::text);


--
-- Name: idx_topic_custom_fields_topic_post_event_ends_at; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_topic_custom_fields_topic_post_event_ends_at ON public.topic_custom_fields USING btree (name, topic_id) WHERE ((name)::text = 'TopicEventEndsAt'::text);


--
-- Name: idx_topic_custom_fields_topic_post_event_starts_at; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_topic_custom_fields_topic_post_event_starts_at ON public.topic_custom_fields USING btree (name, topic_id) WHERE ((name)::text = 'TopicEventStartsAt'::text);


--
-- Name: idx_topic_id_public_type_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_topic_id_public_type_deleted_at ON public.topic_timers USING btree (topic_id) WHERE ((public_type = true) AND (deleted_at IS NULL) AND ((type)::text = 'TopicTimer'::text));


--
-- Name: idx_topics_front_page; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_topics_front_page ON public.topics USING btree (deleted_at, visible, archetype, category_id, id);


--
-- Name: idx_topics_user_id_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_topics_user_id_deleted_at ON public.topics USING btree (user_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_unique_actions; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_unique_actions ON public.post_actions USING btree (user_id, post_action_type_id, post_id, targets_topic) WHERE ((deleted_at IS NULL) AND (disagreed_at IS NULL) AND (deferred_at IS NULL));


--
-- Name: idx_unique_flags; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_unique_flags ON public.post_actions USING btree (user_id, post_id, targets_topic) WHERE ((deleted_at IS NULL) AND (disagreed_at IS NULL) AND (deferred_at IS NULL) AND (post_action_type_id = ANY (ARRAY[3, 4, 7, 8])));


--
-- Name: idx_unique_rows; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_unique_rows ON public.user_actions USING btree (action_type, user_id, target_topic_id, target_post_id, acting_user_id);


--
-- Name: idx_unique_sidebar_section_links; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_unique_sidebar_section_links ON public.sidebar_section_links USING btree (user_id, linkable_type, linkable_id);


--
-- Name: idx_upcoming_change_events_unique_once_off; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_upcoming_change_events_unique_once_off ON public.upcoming_change_events USING btree (upcoming_change_name, event_type) WHERE (event_type = ANY (ARRAY[0, 1, 6, 7]));


--
-- Name: idx_uploads_on_verification_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_uploads_on_verification_status ON public.uploads USING btree (verification_status);


--
-- Name: idx_user_actions_speed_up_user_all; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_actions_speed_up_user_all ON public.user_actions USING btree (user_id, created_at, action_type);


--
-- Name: idx_user_chat_thread_memberships_on_thread_id_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_chat_thread_memberships_on_thread_id_user_id ON public.user_chat_thread_memberships USING btree (thread_id, user_id);


--
-- Name: idx_user_custom_fields_last_reminded_at; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_user_custom_fields_last_reminded_at ON public.user_custom_fields USING btree (name, user_id) WHERE ((name)::text = 'last_reminded_at'::text);


--
-- Name: idx_user_custom_fields_on_holiday; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_user_custom_fields_on_holiday ON public.user_custom_fields USING btree (name, user_id) WHERE ((name)::text = 'on_holiday'::text);


--
-- Name: idx_user_custom_fields_remind_assigns_frequency; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_user_custom_fields_remind_assigns_frequency ON public.user_custom_fields USING btree (name, user_id) WHERE ((name)::text = 'remind_assigns_frequency'::text);


--
-- Name: idx_user_custom_fields_user_notes_count; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_user_custom_fields_user_notes_count ON public.user_custom_fields USING btree (name, user_id) WHERE ((name)::text = 'user_notes_count'::text);


--
-- Name: idx_users_admin; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_admin ON public.users USING btree (id) WHERE admin;


--
-- Name: idx_users_ip_address; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_ip_address ON public.users USING btree (ip_address);


--
-- Name: idx_users_moderator; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_moderator ON public.users USING btree (id) WHERE moderator;


--
-- Name: idx_voice_co_presences_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_voice_co_presences_unique ON public.voice_co_presences USING btree (user_id_1, user_id_2, date);


--
-- Name: idx_voice_invites_redeemed; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_voice_invites_redeemed ON public.voice_invites USING btree (invited_by_id, user_id) WHERE (redeemed_at IS NOT NULL);


--
-- Name: idx_voice_room_memberships_on_room_and_user; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_voice_room_memberships_on_room_and_user ON public.voice_room_memberships USING btree (room_id, user_id);


--
-- Name: idx_voice_sessions_orphaned; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_voice_sessions_orphaned ON public.voice_sessions USING btree (left_at) WHERE (left_at IS NULL);


--
-- Name: idx_web_hook_event_types_hooks_on_ids; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_web_hook_event_types_hooks_on_ids ON public.web_hook_event_types_hooks USING btree (web_hook_event_type_id, web_hook_id);


--
-- Name: idxtopicslug; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idxtopicslug ON public.topics USING btree (slug) WHERE ((deleted_at IS NULL) AND (slug IS NOT NULL));


--
-- Name: index_ad_plugin_house_ads_on_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_ad_plugin_house_ads_on_name ON public.ad_plugin_house_ads USING btree (name);


--
-- Name: index_ad_plugin_house_ads_on_visible_to_anons; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ad_plugin_house_ads_on_visible_to_anons ON public.ad_plugin_house_ads USING btree (visible_to_anons);


--
-- Name: index_ad_plugin_house_ads_on_visible_to_logged_in_users; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ad_plugin_house_ads_on_visible_to_logged_in_users ON public.ad_plugin_house_ads USING btree (visible_to_logged_in_users);


--
-- Name: index_ad_plugin_impressions_on_ad_plugin_house_ad_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ad_plugin_impressions_on_ad_plugin_house_ad_id ON public.ad_plugin_impressions USING btree (ad_plugin_house_ad_id);


--
-- Name: index_ad_plugin_impressions_on_ad_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ad_plugin_impressions_on_ad_type ON public.ad_plugin_impressions USING btree (ad_type);


--
-- Name: index_ad_plugin_impressions_on_ad_type_and_placement; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ad_plugin_impressions_on_ad_type_and_placement ON public.ad_plugin_impressions USING btree (ad_type, placement);


--
-- Name: index_ad_plugin_impressions_on_clicked_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ad_plugin_impressions_on_clicked_at ON public.ad_plugin_impressions USING btree (clicked_at);


--
-- Name: index_ad_plugin_impressions_on_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ad_plugin_impressions_on_created_at ON public.ad_plugin_impressions USING btree (created_at);


--
-- Name: index_ad_plugin_impressions_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ad_plugin_impressions_on_user_id ON public.ad_plugin_impressions USING btree (user_id);


--
-- Name: index_admin_dashboard_reports_on_position; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_admin_dashboard_reports_on_position ON public.admin_dashboard_reports USING btree ("position");


--
-- Name: index_admin_dashboard_reports_on_source_and_identifier; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_admin_dashboard_reports_on_source_and_identifier ON public.admin_dashboard_reports USING btree (source, identifier);


--
-- Name: index_admin_dashboard_sections_on_section_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_admin_dashboard_sections_on_section_id ON public.admin_dashboard_sections USING btree (section_id);


--
-- Name: index_admin_notices_on_identifier; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_admin_notices_on_identifier ON public.admin_notices USING btree (identifier);


--
-- Name: index_admin_notices_on_subject; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_admin_notices_on_subject ON public.admin_notices USING btree (subject);


--
-- Name: index_ai_agent_mcp_servers_on_ai_agent_id_and_ai_mcp_server_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_ai_agent_mcp_servers_on_ai_agent_id_and_ai_mcp_server_id ON public.ai_agent_mcp_servers USING btree (ai_agent_id, ai_mcp_server_id);


--
-- Name: index_ai_agent_mcp_servers_on_ai_mcp_server_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ai_agent_mcp_servers_on_ai_mcp_server_id ON public.ai_agent_mcp_servers USING btree (ai_mcp_server_id);


--
-- Name: index_ai_agents_on_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_ai_agents_on_name ON public.ai_agents USING btree (name);


--
-- Name: index_ai_api_audit_logs_on_created_at_and_feature_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ai_api_audit_logs_on_created_at_and_feature_name ON public.ai_api_audit_logs USING btree (created_at, feature_name);


--
-- Name: index_ai_api_audit_logs_on_created_at_and_language_model; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ai_api_audit_logs_on_created_at_and_language_model ON public.ai_api_audit_logs USING btree (created_at, language_model);


--
-- Name: index_ai_api_audit_logs_on_created_at_and_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ai_api_audit_logs_on_created_at_and_user_id ON public.ai_api_audit_logs USING btree (created_at, user_id);


--
-- Name: index_ai_api_audit_logs_on_llm_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ai_api_audit_logs_on_llm_id ON public.ai_api_audit_logs USING btree (llm_id);


--
-- Name: index_ai_api_audit_logs_on_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ai_api_audit_logs_on_post_id ON public.ai_api_audit_logs USING btree (post_id);


--
-- Name: index_ai_api_audit_logs_on_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ai_api_audit_logs_on_topic_id ON public.ai_api_audit_logs USING btree (topic_id);


--
-- Name: index_ai_api_request_stats_on_bucket_date_and_feature_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ai_api_request_stats_on_bucket_date_and_feature_name ON public.ai_api_request_stats USING btree (bucket_date, feature_name);


--
-- Name: index_ai_api_request_stats_on_bucket_date_and_language_model; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ai_api_request_stats_on_bucket_date_and_language_model ON public.ai_api_request_stats USING btree (bucket_date, language_model);


--
-- Name: index_ai_api_request_stats_on_bucket_date_and_llm_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ai_api_request_stats_on_bucket_date_and_llm_id ON public.ai_api_request_stats USING btree (bucket_date, llm_id);


--
-- Name: index_ai_api_request_stats_on_bucket_date_and_rolled_up; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ai_api_request_stats_on_bucket_date_and_rolled_up ON public.ai_api_request_stats USING btree (bucket_date, rolled_up) WHERE (rolled_up = false);


--
-- Name: index_ai_api_request_stats_on_bucket_date_and_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ai_api_request_stats_on_bucket_date_and_user_id ON public.ai_api_request_stats USING btree (bucket_date, user_id);


--
-- Name: index_ai_api_request_stats_on_created_at_and_feature_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ai_api_request_stats_on_created_at_and_feature_name ON public.ai_api_request_stats USING btree (created_at, feature_name);


--
-- Name: index_ai_api_request_stats_on_created_at_and_language_model; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ai_api_request_stats_on_created_at_and_language_model ON public.ai_api_request_stats USING btree (created_at, language_model);


--
-- Name: index_ai_api_request_stats_on_created_at_and_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ai_api_request_stats_on_created_at_and_user_id ON public.ai_api_request_stats USING btree (created_at, user_id);


--
-- Name: index_ai_artifact_kv_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_ai_artifact_kv_unique ON public.ai_artifact_key_values USING btree (ai_artifact_id, user_id, key);


--
-- Name: index_ai_artifact_versions_on_ai_artifact_id_and_version_number; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_ai_artifact_versions_on_ai_artifact_id_and_version_number ON public.ai_artifact_versions USING btree (ai_artifact_id, version_number);


--
-- Name: index_ai_fragments_embeddings_on_model_strategy_fragment; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_ai_fragments_embeddings_on_model_strategy_fragment ON public.ai_document_fragments_embeddings USING btree (model_id, strategy_id, rag_document_fragment_id);


--
-- Name: index_ai_mcp_oauth_tokens_on_ai_mcp_server_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_ai_mcp_oauth_tokens_on_ai_mcp_server_id ON public.ai_mcp_oauth_tokens USING btree (ai_mcp_server_id);


--
-- Name: index_ai_mcp_servers_on_ai_secret_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ai_mcp_servers_on_ai_secret_id ON public.ai_mcp_servers USING btree (ai_secret_id);


--
-- Name: index_ai_mcp_servers_on_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_ai_mcp_servers_on_name ON public.ai_mcp_servers USING btree (name);


--
-- Name: index_ai_mcp_servers_on_oauth_client_secret_ai_secret_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ai_mcp_servers_on_oauth_client_secret_ai_secret_id ON public.ai_mcp_servers USING btree (oauth_client_secret_ai_secret_id);


--
-- Name: index_ai_moderation_settings_on_setting_type; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_ai_moderation_settings_on_setting_type ON public.ai_moderation_settings USING btree (setting_type);


--
-- Name: index_ai_posts_embeddings_on_model_strategy_post; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_ai_posts_embeddings_on_model_strategy_post ON public.ai_posts_embeddings USING btree (model_id, strategy_id, post_id);


--
-- Name: index_ai_secrets_on_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_ai_secrets_on_name ON public.ai_secrets USING btree (name);


--
-- Name: index_ai_spam_logs_on_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ai_spam_logs_on_post_id ON public.ai_spam_logs USING btree (post_id);


--
-- Name: index_ai_summaries_on_target_type_and_target_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ai_summaries_on_target_type_and_target_id ON public.ai_summaries USING btree (target_type, target_id);


--
-- Name: index_ai_tool_actions_on_ai_agent_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ai_tool_actions_on_ai_agent_id ON public.ai_tool_actions USING btree (ai_agent_id);


--
-- Name: index_ai_tool_secret_bindings_on_ai_secret_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ai_tool_secret_bindings_on_ai_secret_id ON public.ai_tool_secret_bindings USING btree (ai_secret_id);


--
-- Name: index_ai_tool_secret_bindings_on_ai_tool_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ai_tool_secret_bindings_on_ai_tool_id ON public.ai_tool_secret_bindings USING btree (ai_tool_id);


--
-- Name: index_ai_tool_secret_bindings_on_ai_tool_id_and_alias; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_ai_tool_secret_bindings_on_ai_tool_id_and_alias ON public.ai_tool_secret_bindings USING btree (ai_tool_id, alias);


--
-- Name: index_ai_topics_embeddings_on_model_strategy_topic; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_ai_topics_embeddings_on_model_strategy_topic ON public.ai_topics_embeddings USING btree (model_id, strategy_id, topic_id);


--
-- Name: index_ai_topics_embeddings_on_topic_id_and_model_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ai_topics_embeddings_on_topic_id_and_model_id ON public.ai_topics_embeddings USING btree (topic_id, model_id);


--
-- Name: index_allowed_pm_users_on_allowed_pm_user_id_and_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_allowed_pm_users_on_allowed_pm_user_id_and_user_id ON public.allowed_pm_users USING btree (allowed_pm_user_id, user_id);


--
-- Name: index_allowed_pm_users_on_user_id_and_allowed_pm_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_allowed_pm_users_on_user_id_and_allowed_pm_user_id ON public.allowed_pm_users USING btree (user_id, allowed_pm_user_id);


--
-- Name: index_anonymous_users_on_master_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_anonymous_users_on_master_user_id ON public.anonymous_users USING btree (master_user_id) WHERE active;


--
-- Name: index_anonymous_users_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_anonymous_users_on_user_id ON public.anonymous_users USING btree (user_id);


--
-- Name: index_api_key_scopes_on_api_key_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_api_key_scopes_on_api_key_id ON public.api_key_scopes USING btree (api_key_id);


--
-- Name: index_api_keys_on_key_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_api_keys_on_key_hash ON public.api_keys USING btree (key_hash);


--
-- Name: index_api_keys_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_api_keys_on_user_id ON public.api_keys USING btree (user_id);


--
-- Name: index_application_requests_on_date_and_req_type; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_application_requests_on_date_and_req_type ON public.application_requests USING btree (date, req_type);


--
-- Name: index_ask_ai_logs_on_asked_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ask_ai_logs_on_asked_at ON public.ask_ai_logs USING btree (asked_at);


--
-- Name: index_ask_ai_logs_on_user_id_and_asked_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_ask_ai_logs_on_user_id_and_asked_at ON public.ask_ai_logs USING btree (user_id, asked_at);


--
-- Name: index_assignments_on_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_assignments_on_active ON public.assignments USING btree (active);


--
-- Name: index_assignments_on_assigned_to_id_and_assigned_to_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_assignments_on_assigned_to_id_and_assigned_to_type ON public.assignments USING btree (assigned_to_id, assigned_to_type);


--
-- Name: index_assignments_on_target_id_and_target_type; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_assignments_on_target_id_and_target_type ON public.assignments USING btree (target_id, target_type);


--
-- Name: index_assignments_on_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_assignments_on_topic_id ON public.assignments USING btree (topic_id);


--
-- Name: index_backup_draft_posts_on_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_backup_draft_posts_on_post_id ON public.backup_draft_posts USING btree (post_id);


--
-- Name: index_backup_draft_posts_on_user_id_and_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_backup_draft_posts_on_user_id_and_key ON public.backup_draft_posts USING btree (user_id, key);


--
-- Name: index_backup_draft_topics_on_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_backup_draft_topics_on_topic_id ON public.backup_draft_topics USING btree (topic_id);


--
-- Name: index_backup_draft_topics_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_backup_draft_topics_on_user_id ON public.backup_draft_topics USING btree (user_id);


--
-- Name: index_badge_types_on_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_badge_types_on_name ON public.badge_types USING btree (name);


--
-- Name: index_badges_on_badge_type_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_badges_on_badge_type_id ON public.badges USING btree (badge_type_id);


--
-- Name: index_badges_on_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_badges_on_name ON public.badges USING btree (name);


--
-- Name: index_bookmarks_on_reminder_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_bookmarks_on_reminder_at ON public.bookmarks USING btree (reminder_at);


--
-- Name: index_bookmarks_on_reminder_set_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_bookmarks_on_reminder_set_at ON public.bookmarks USING btree (reminder_set_at);


--
-- Name: index_bookmarks_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_bookmarks_on_user_id ON public.bookmarks USING btree (user_id);


--
-- Name: index_browser_pageview_event_scores_on_event_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_browser_pageview_event_scores_on_event_id ON public.browser_pageview_event_scores USING btree (event_id);


--
-- Name: index_browser_pageview_events_on_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_browser_pageview_events_on_created_at ON public.browser_pageview_events USING brin (created_at);


--
-- Name: index_browser_pageview_events_on_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_browser_pageview_events_on_topic_id ON public.browser_pageview_events USING btree (topic_id);


--
-- Name: index_browser_pageview_events_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_browser_pageview_events_on_user_id ON public.browser_pageview_events USING btree (user_id);


--
-- Name: index_browser_pageview_session_engagements_on_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_browser_pageview_session_engagements_on_created_at ON public.browser_pageview_session_engagements USING brin (created_at);


--
-- Name: index_browser_pageview_session_engagements_on_session_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_browser_pageview_session_engagements_on_session_id ON public.browser_pageview_session_engagements USING btree (session_id);


--
-- Name: index_calendar_events_on_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_calendar_events_on_post_id ON public.calendar_events USING btree (post_id);


--
-- Name: index_calendar_events_on_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_calendar_events_on_topic_id ON public.calendar_events USING btree (topic_id);


--
-- Name: index_calendar_events_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_calendar_events_on_user_id ON public.calendar_events USING btree (user_id);


--
-- Name: index_categories_on_email_in; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_categories_on_email_in ON public.categories USING btree (email_in);


--
-- Name: index_categories_on_reviewable_by_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_categories_on_reviewable_by_group_id ON public.categories USING btree (reviewable_by_group_id);


--
-- Name: index_categories_on_search_priority; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_categories_on_search_priority ON public.categories USING btree (search_priority);


--
-- Name: index_categories_on_topic_count; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_categories_on_topic_count ON public.categories USING btree (topic_count);


--
-- Name: index_categories_web_hooks_on_web_hook_id_and_category_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_categories_web_hooks_on_web_hook_id_and_category_id ON public.categories_web_hooks USING btree (web_hook_id, category_id);


--
-- Name: index_category_activity_daily_rollups_on_date_and_category_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_category_activity_daily_rollups_on_date_and_category_id ON public.category_activity_daily_rollups USING btree (date, category_id);


--
-- Name: index_category_custom_fields_on_category_id_and_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_category_custom_fields_on_category_id_and_name ON public.category_custom_fields USING btree (category_id, name);


--
-- Name: index_category_featured_topics_on_category_id_and_rank; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_category_featured_topics_on_category_id_and_rank ON public.category_featured_topics USING btree (category_id, rank);


--
-- Name: index_category_form_templates_on_category_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_category_form_templates_on_category_id ON public.category_form_templates USING btree (category_id);


--
-- Name: index_category_form_templates_on_form_template_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_category_form_templates_on_form_template_id ON public.category_form_templates USING btree (form_template_id);


--
-- Name: index_category_groups_on_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_category_groups_on_group_id ON public.category_groups USING btree (group_id);


--
-- Name: index_category_localizations_on_category_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_category_localizations_on_category_id ON public.category_localizations USING btree (category_id);


--
-- Name: index_category_localizations_on_category_id_and_locale; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_category_localizations_on_category_id_and_locale ON public.category_localizations USING btree (category_id, locale);


--
-- Name: index_category_moderation_groups_on_category_id_and_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_category_moderation_groups_on_category_id_and_group_id ON public.category_moderation_groups USING btree (category_id, group_id);


--
-- Name: index_category_settings_on_category_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_category_settings_on_category_id ON public.category_settings USING btree (category_id);


--
-- Name: index_category_tag_stats_on_category_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_category_tag_stats_on_category_id ON public.category_tag_stats USING btree (category_id);


--
-- Name: index_category_tag_stats_on_category_id_and_tag_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_category_tag_stats_on_category_id_and_tag_id ON public.category_tag_stats USING btree (category_id, tag_id);


--
-- Name: index_category_tag_stats_on_category_id_and_topic_count; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_category_tag_stats_on_category_id_and_topic_count ON public.category_tag_stats USING btree (category_id, topic_count);


--
-- Name: index_category_tag_stats_on_tag_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_category_tag_stats_on_tag_id ON public.category_tag_stats USING btree (tag_id);


--
-- Name: index_category_users_on_category_id_and_notification_level; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_category_users_on_category_id_and_notification_level ON public.category_users USING btree (category_id, notification_level);


--
-- Name: index_category_users_on_user_id_and_last_seen_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_category_users_on_user_id_and_last_seen_at ON public.category_users USING btree (user_id, last_seen_at);


--
-- Name: index_chat_channel_archives_on_chat_channel_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_channel_archives_on_chat_channel_id ON public.chat_channel_archives USING btree (chat_channel_id);


--
-- Name: index_chat_channel_custom_fields_on_channel_id_and_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_chat_channel_custom_fields_on_channel_id_and_name ON public.chat_channel_custom_fields USING btree (channel_id, name);


--
-- Name: index_chat_channels_on_chatable_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_channels_on_chatable_id ON public.chat_channels USING btree (chatable_id);


--
-- Name: index_chat_channels_on_chatable_id_and_chatable_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_channels_on_chatable_id_and_chatable_type ON public.chat_channels USING btree (chatable_id, chatable_type);


--
-- Name: index_chat_channels_on_last_message_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_channels_on_last_message_id ON public.chat_channels USING btree (last_message_id);


--
-- Name: index_chat_channels_on_messages_count; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_channels_on_messages_count ON public.chat_channels USING btree (messages_count);


--
-- Name: index_chat_channels_on_slug; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_chat_channels_on_slug ON public.chat_channels USING btree (slug) WHERE ((slug)::text <> ''::text);


--
-- Name: index_chat_channels_on_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_channels_on_status ON public.chat_channels USING btree (status);


--
-- Name: index_chat_mention_notifications_on_chat_mention_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_mention_notifications_on_chat_mention_id ON public.chat_mention_notifications USING btree (chat_mention_id);


--
-- Name: index_chat_mention_notifications_on_notification_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_chat_mention_notifications_on_notification_id ON public.chat_mention_notifications USING btree (notification_id);


--
-- Name: index_chat_mentions_on_chat_message_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_mentions_on_chat_message_id ON public.chat_mentions USING btree (chat_message_id);


--
-- Name: index_chat_mentions_on_target_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_mentions_on_target_id ON public.chat_mentions USING btree (target_id);


--
-- Name: index_chat_message_custom_fields_on_message_id_and_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_chat_message_custom_fields_on_message_id_and_name ON public.chat_message_custom_fields USING btree (message_id, name);


--
-- Name: index_chat_message_custom_prompts_on_message_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_chat_message_custom_prompts_on_message_id ON public.chat_message_custom_prompts USING btree (message_id);


--
-- Name: index_chat_message_hotlinked_media_on_message_and_url_md5; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_chat_message_hotlinked_media_on_message_and_url_md5 ON public.chat_message_hotlinked_media USING btree (chat_message_id, md5((url)::text));


--
-- Name: index_chat_message_hotlinked_media_on_upload_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_message_hotlinked_media_on_upload_id ON public.chat_message_hotlinked_media USING btree (upload_id);


--
-- Name: index_chat_message_interactions_on_chat_message_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_message_interactions_on_chat_message_id ON public.chat_message_interactions USING btree (chat_message_id);


--
-- Name: index_chat_message_interactions_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_message_interactions_on_user_id ON public.chat_message_interactions USING btree (user_id);


--
-- Name: index_chat_message_links_on_chat_message_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_message_links_on_chat_message_id ON public.chat_message_links USING btree (chat_message_id);


--
-- Name: index_chat_message_links_on_url; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_message_links_on_url ON public.chat_message_links USING btree (url);


--
-- Name: index_chat_message_revisions_on_chat_message_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_message_revisions_on_chat_message_id ON public.chat_message_revisions USING btree (chat_message_id);


--
-- Name: index_chat_message_revisions_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_message_revisions_on_user_id ON public.chat_message_revisions USING btree (user_id);


--
-- Name: index_chat_messages_on_chat_channel_id_and_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_messages_on_chat_channel_id_and_created_at ON public.chat_messages USING btree (chat_channel_id, created_at);


--
-- Name: index_chat_messages_on_chat_channel_id_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_messages_on_chat_channel_id_and_id ON public.chat_messages USING btree (chat_channel_id, id) WHERE (deleted_at IS NOT NULL);


--
-- Name: index_chat_messages_on_last_editor_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_messages_on_last_editor_id ON public.chat_messages USING btree (last_editor_id);


--
-- Name: index_chat_messages_on_thread_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_messages_on_thread_id ON public.chat_messages USING btree (thread_id);


--
-- Name: index_chat_pinned_messages_on_chat_message_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_chat_pinned_messages_on_chat_message_id ON public.chat_pinned_messages USING btree (chat_message_id);


--
-- Name: index_chat_thread_custom_fields_on_thread_id_and_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_chat_thread_custom_fields_on_thread_id_and_name ON public.chat_thread_custom_fields USING btree (thread_id, name);


--
-- Name: index_chat_threads_on_channel_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_threads_on_channel_id ON public.chat_threads USING btree (channel_id);


--
-- Name: index_chat_threads_on_channel_id_and_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_threads_on_channel_id_and_status ON public.chat_threads USING btree (channel_id, status);


--
-- Name: index_chat_threads_on_last_message_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_threads_on_last_message_id ON public.chat_threads USING btree (last_message_id);


--
-- Name: index_chat_threads_on_original_message_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_threads_on_original_message_id ON public.chat_threads USING btree (original_message_id);


--
-- Name: index_chat_threads_on_original_message_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_threads_on_original_message_user_id ON public.chat_threads USING btree (original_message_user_id);


--
-- Name: index_chat_threads_on_replies_count; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_threads_on_replies_count ON public.chat_threads USING btree (replies_count);


--
-- Name: index_chat_threads_on_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_chat_threads_on_status ON public.chat_threads USING btree (status);


--
-- Name: index_child_themes_on_child_theme_id_and_parent_theme_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_child_themes_on_child_theme_id_and_parent_theme_id ON public.child_themes USING btree (child_theme_id, parent_theme_id);


--
-- Name: index_child_themes_on_parent_theme_id_and_child_theme_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_child_themes_on_parent_theme_id_and_child_theme_id ON public.child_themes USING btree (parent_theme_id, child_theme_id);


--
-- Name: index_color_scheme_colors_on_color_scheme_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_color_scheme_colors_on_color_scheme_id ON public.color_scheme_colors USING btree (color_scheme_id);


--
-- Name: index_completion_prompts_on_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_completion_prompts_on_name ON public.completion_prompts USING btree (name);


--
-- Name: index_custom_emojis_on_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_custom_emojis_on_name ON public.custom_emojis USING btree (name);


--
-- Name: index_data_explorer_query_groups_on_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_data_explorer_query_groups_on_group_id ON public.data_explorer_query_groups USING btree (group_id);


--
-- Name: index_data_explorer_query_groups_on_query_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_data_explorer_query_groups_on_query_id ON public.data_explorer_query_groups USING btree (query_id);


--
-- Name: index_data_explorer_query_groups_on_query_id_and_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_data_explorer_query_groups_on_query_id_and_group_id ON public.data_explorer_query_groups USING btree (query_id, group_id);


--
-- Name: index_data_explorer_query_stats_on_query_id_and_date; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_data_explorer_query_stats_on_query_id_and_date ON public.data_explorer_query_stats USING btree (query_id, date);


--
-- Name: index_developers_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_developers_on_user_id ON public.developers USING btree (user_id);


--
-- Name: index_directory_items_on_days_visited; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_directory_items_on_days_visited ON public.directory_items USING btree (days_visited);


--
-- Name: index_directory_items_on_likes_given; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_directory_items_on_likes_given ON public.directory_items USING btree (likes_given);


--
-- Name: index_directory_items_on_likes_received; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_directory_items_on_likes_received ON public.directory_items USING btree (likes_received);


--
-- Name: index_directory_items_on_period_type_and_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_directory_items_on_period_type_and_user_id ON public.directory_items USING btree (period_type, user_id);


--
-- Name: index_directory_items_on_post_count; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_directory_items_on_post_count ON public.directory_items USING btree (post_count);


--
-- Name: index_directory_items_on_posts_read; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_directory_items_on_posts_read ON public.directory_items USING btree (posts_read);


--
-- Name: index_directory_items_on_topic_count; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_directory_items_on_topic_count ON public.directory_items USING btree (topic_count);


--
-- Name: index_directory_items_on_topics_entered; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_directory_items_on_topics_entered ON public.directory_items USING btree (topics_entered);


--
-- Name: index_disabled_holidays_on_holiday_name_and_region_code; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_disabled_holidays_on_holiday_name_and_region_code ON public.discourse_calendar_disabled_holidays USING btree (holiday_name, region_code);


--
-- Name: index_discourse_automation_stats_on_automation_id_and_date; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_discourse_automation_stats_on_automation_id_and_date ON public.discourse_automation_stats USING btree (automation_id, date);


--
-- Name: index_discourse_calendar_post_event_dates_on_event_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_discourse_calendar_post_event_dates_on_event_id ON public.discourse_calendar_post_event_dates USING btree (event_id);


--
-- Name: index_discourse_calendar_post_event_dates_on_event_id_and_dates; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_discourse_calendar_post_event_dates_on_event_id_and_dates ON public.discourse_calendar_post_event_dates USING btree (event_id, finished_at, starts_at DESC, updated_at DESC, id DESC);


--
-- Name: index_discourse_calendar_post_event_dates_on_finished_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_discourse_calendar_post_event_dates_on_finished_at ON public.discourse_calendar_post_event_dates USING btree (finished_at);


--
-- Name: index_discourse_kanban_board_histories_on_acting_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_discourse_kanban_board_histories_on_acting_user_id ON public.discourse_kanban_board_histories USING btree (acting_user_id);


--
-- Name: index_discourse_kanban_board_histories_on_board_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_discourse_kanban_board_histories_on_board_id ON public.discourse_kanban_board_histories USING btree (board_id);


--
-- Name: index_discourse_kanban_board_histories_on_column_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_discourse_kanban_board_histories_on_column_id ON public.discourse_kanban_board_histories USING btree (column_id);


--
-- Name: index_discourse_kanban_board_histories_one_view_per_user_day; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_discourse_kanban_board_histories_one_view_per_user_day ON public.discourse_kanban_board_histories USING btree (board_id, acting_user_id, ((created_at)::date)) WHERE (action = 7);


--
-- Name: index_discourse_kanban_boards_on_created_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_discourse_kanban_boards_on_created_by_id ON public.discourse_kanban_boards USING btree (created_by_id);


--
-- Name: index_discourse_kanban_boards_on_slug; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_discourse_kanban_boards_on_slug ON public.discourse_kanban_boards USING btree (slug);


--
-- Name: index_discourse_kanban_card_histories_on_acting_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_discourse_kanban_card_histories_on_acting_user_id ON public.discourse_kanban_card_histories USING btree (acting_user_id);


--
-- Name: index_discourse_kanban_card_histories_on_board_id_and_card_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_discourse_kanban_card_histories_on_board_id_and_card_id ON public.discourse_kanban_card_histories USING btree (board_id, card_id);


--
-- Name: index_discourse_kanban_card_histories_one_view_per_user_day; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_discourse_kanban_card_histories_one_view_per_user_day ON public.discourse_kanban_card_histories USING btree (board_id, card_id, acting_user_id, ((created_at)::date)) WHERE (action = 7);


--
-- Name: index_discourse_post_event_events_on_image_upload_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_discourse_post_event_events_on_image_upload_id ON public.discourse_post_event_events USING btree (image_upload_id);


--
-- Name: index_discourse_post_event_hosts_on_post_id_and_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_discourse_post_event_hosts_on_post_id_and_user_id ON public.discourse_post_event_hosts USING btree (post_id, user_id);


--
-- Name: index_discourse_post_event_hosts_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_discourse_post_event_hosts_on_user_id ON public.discourse_post_event_hosts USING btree (user_id);


--
-- Name: index_discourse_reactions_reaction_users_on_reaction_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_discourse_reactions_reaction_users_on_reaction_id ON public.discourse_reactions_reaction_users USING btree (reaction_id);


--
-- Name: index_discourse_reactions_reactions_on_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_discourse_reactions_reactions_on_post_id ON public.discourse_reactions_reactions USING btree (post_id);


--
-- Name: index_discourse_rss_polling_rss_feeds_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_discourse_rss_polling_rss_feeds_on_user_id ON public.discourse_rss_polling_rss_feeds USING btree (user_id);


--
-- Name: index_discourse_solved_shared_issues_on_topic_id_and_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_discourse_solved_shared_issues_on_topic_id_and_user_id ON public.discourse_solved_shared_issues USING btree (topic_id, user_id);


--
-- Name: index_discourse_solved_shared_issues_on_user_id_and_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_discourse_solved_shared_issues_on_user_id_and_topic_id ON public.discourse_solved_shared_issues USING btree (user_id, topic_id);


--
-- Name: index_discourse_solved_solved_topics_on_answer_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_discourse_solved_solved_topics_on_answer_post_id ON public.discourse_solved_solved_topics USING btree (answer_post_id);


--
-- Name: index_discourse_solved_solved_topics_on_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_discourse_solved_solved_topics_on_topic_id ON public.discourse_solved_solved_topics USING btree (topic_id);


--
-- Name: index_discourse_solved_topic_answers_on_answer_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_discourse_solved_topic_answers_on_answer_post_id ON public.discourse_solved_topic_answers USING btree (answer_post_id);


--
-- Name: index_discourse_solved_topic_answers_on_solved_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_discourse_solved_topic_answers_on_solved_topic_id ON public.discourse_solved_topic_answers USING btree (solved_topic_id);


--
-- Name: index_discourse_subscriptions_customers_on_customer_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_discourse_subscriptions_customers_on_customer_id ON public.discourse_subscriptions_customers USING btree (customer_id);


--
-- Name: index_discourse_subscriptions_customers_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_discourse_subscriptions_customers_on_user_id ON public.discourse_subscriptions_customers USING btree (user_id);


--
-- Name: index_discourse_subscriptions_products_on_external_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_discourse_subscriptions_products_on_external_id ON public.discourse_subscriptions_products USING btree (external_id);


--
-- Name: index_discourse_subscriptions_subscriptions_on_customer_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_discourse_subscriptions_subscriptions_on_customer_id ON public.discourse_subscriptions_subscriptions USING btree (customer_id);


--
-- Name: index_discourse_subscriptions_subscriptions_on_external_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_discourse_subscriptions_subscriptions_on_external_id ON public.discourse_subscriptions_subscriptions USING btree (external_id);


--
-- Name: index_discourse_templates_usage_count_on_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_discourse_templates_usage_count_on_topic_id ON public.discourse_templates_usage_count USING btree (topic_id);


--
-- Name: index_dismissed_topic_users_on_user_id_and_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_dismissed_topic_users_on_user_id_and_topic_id ON public.dismissed_topic_users USING btree (user_id, topic_id);


--
-- Name: index_do_not_disturb_timings_on_ends_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_do_not_disturb_timings_on_ends_at ON public.do_not_disturb_timings USING btree (ends_at);


--
-- Name: index_do_not_disturb_timings_on_scheduled; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_do_not_disturb_timings_on_scheduled ON public.do_not_disturb_timings USING btree (scheduled);


--
-- Name: index_do_not_disturb_timings_on_starts_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_do_not_disturb_timings_on_starts_at ON public.do_not_disturb_timings USING btree (starts_at);


--
-- Name: index_do_not_disturb_timings_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_do_not_disturb_timings_on_user_id ON public.do_not_disturb_timings USING btree (user_id);


--
-- Name: index_draft_sequences_on_user_id_and_draft_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_draft_sequences_on_user_id_and_draft_key ON public.draft_sequences USING btree (user_id, draft_key);


--
-- Name: index_drafts_on_user_id_and_draft_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_drafts_on_user_id_and_draft_key ON public.drafts USING btree (user_id, draft_key);


--
-- Name: index_email_change_requests_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_email_change_requests_on_user_id ON public.email_change_requests USING btree (user_id);


--
-- Name: index_email_login_codes_on_expires_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_email_login_codes_on_expires_at ON public.email_login_codes USING btree (expires_at);


--
-- Name: index_email_login_codes_on_lower_email; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_email_login_codes_on_lower_email ON public.email_login_codes USING btree (lower((email)::text));


--
-- Name: index_email_logs_on_bounce_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_email_logs_on_bounce_key ON public.email_logs USING btree (bounce_key) WHERE (bounce_key IS NOT NULL);


--
-- Name: index_email_logs_on_bounced; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_email_logs_on_bounced ON public.email_logs USING btree (bounced);


--
-- Name: index_email_logs_on_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_email_logs_on_created_at ON public.email_logs USING btree (created_at DESC);


--
-- Name: index_email_logs_on_message_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_email_logs_on_message_id ON public.email_logs USING btree (message_id);


--
-- Name: index_email_logs_on_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_email_logs_on_post_id ON public.email_logs USING btree (post_id);


--
-- Name: index_email_logs_on_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_email_logs_on_topic_id ON public.email_logs USING btree (topic_id) WHERE (topic_id IS NOT NULL);


--
-- Name: index_email_logs_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_email_logs_on_user_id ON public.email_logs USING btree (user_id);


--
-- Name: index_email_tokens_on_token_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_email_tokens_on_token_hash ON public.email_tokens USING btree (token_hash);


--
-- Name: index_email_tokens_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_email_tokens_on_user_id ON public.email_tokens USING btree (user_id);


--
-- Name: index_embeddable_host_tags_on_embeddable_host_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_embeddable_host_tags_on_embeddable_host_id ON public.embeddable_host_tags USING btree (embeddable_host_id);


--
-- Name: index_embeddable_host_tags_on_embeddable_host_id_and_tag_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_embeddable_host_tags_on_embeddable_host_id_and_tag_id ON public.embeddable_host_tags USING btree (embeddable_host_id, tag_id);


--
-- Name: index_embeddable_host_tags_on_tag_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_embeddable_host_tags_on_tag_id ON public.embeddable_host_tags USING btree (tag_id);


--
-- Name: index_embedding_definitions_on_ai_secret_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_embedding_definitions_on_ai_secret_id ON public.embedding_definitions USING btree (ai_secret_id);


--
-- Name: index_external_upload_stubs_on_created_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_external_upload_stubs_on_created_by_id ON public.external_upload_stubs USING btree (created_by_id);


--
-- Name: index_external_upload_stubs_on_external_upload_identifier; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_external_upload_stubs_on_external_upload_identifier ON public.external_upload_stubs USING btree (external_upload_identifier);


--
-- Name: index_external_upload_stubs_on_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_external_upload_stubs_on_key ON public.external_upload_stubs USING btree (key);


--
-- Name: index_external_upload_stubs_on_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_external_upload_stubs_on_status ON public.external_upload_stubs USING btree (status);


--
-- Name: index_flags_on_name_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_flags_on_name_key ON public.flags USING btree (name_key);


--
-- Name: index_for_rebake_old; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_for_rebake_old ON public.posts USING btree (id DESC) WHERE (((baked_version IS NULL) OR (baked_version < 2)) AND (deleted_at IS NULL));


--
-- Name: index_form_templates_on_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_form_templates_on_name ON public.form_templates USING btree (name);


--
-- Name: index_gamification_leaderboards_on_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_gamification_leaderboards_on_name ON public.gamification_leaderboards USING btree (name);


--
-- Name: index_gamification_score_events_on_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_gamification_score_events_on_date ON public.gamification_score_events USING btree (date);


--
-- Name: index_gamification_score_events_on_user_id_and_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_gamification_score_events_on_user_id_and_date ON public.gamification_score_events USING btree (user_id, date);


--
-- Name: index_gamification_scores_on_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_gamification_scores_on_date ON public.gamification_scores USING btree (date);


--
-- Name: index_gamification_scores_on_user_id_and_date; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_gamification_scores_on_user_id_and_date ON public.gamification_scores USING btree (user_id, date);


--
-- Name: index_github_commits_on_repo_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_github_commits_on_repo_id ON public.github_commits USING btree (repo_id);


--
-- Name: index_github_repos_on_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_github_repos_on_name ON public.github_repos USING btree (name);


--
-- Name: index_given_daily_likes_on_limit_reached_and_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_given_daily_likes_on_limit_reached_and_user_id ON public.given_daily_likes USING btree (limit_reached, user_id);


--
-- Name: index_given_daily_likes_on_user_id_and_given_date; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_given_daily_likes_on_user_id_and_given_date ON public.given_daily_likes USING btree (user_id, given_date);


--
-- Name: index_group_archived_messages_on_group_id_and_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_group_archived_messages_on_group_id_and_topic_id ON public.group_archived_messages USING btree (group_id, topic_id);


--
-- Name: index_group_associated_groups; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_group_associated_groups ON public.group_associated_groups USING btree (group_id, associated_group_id);


--
-- Name: index_group_associated_groups_on_associated_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_group_associated_groups_on_associated_group_id ON public.group_associated_groups USING btree (associated_group_id);


--
-- Name: index_group_associated_groups_on_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_group_associated_groups_on_group_id ON public.group_associated_groups USING btree (group_id);


--
-- Name: index_group_custom_fields_on_group_id_and_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_group_custom_fields_on_group_id_and_name ON public.group_custom_fields USING btree (group_id, name);


--
-- Name: index_group_histories_on_acting_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_group_histories_on_acting_user_id ON public.group_histories USING btree (acting_user_id);


--
-- Name: index_group_histories_on_action; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_group_histories_on_action ON public.group_histories USING btree (action);


--
-- Name: index_group_histories_on_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_group_histories_on_group_id ON public.group_histories USING btree (group_id);


--
-- Name: index_group_histories_on_target_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_group_histories_on_target_user_id ON public.group_histories USING btree (target_user_id);


--
-- Name: index_group_mentions_on_group_id_and_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_group_mentions_on_group_id_and_post_id ON public.group_mentions USING btree (group_id, post_id);


--
-- Name: index_group_mentions_on_post_id_and_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_group_mentions_on_post_id_and_group_id ON public.group_mentions USING btree (post_id, group_id);


--
-- Name: index_group_requests_on_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_group_requests_on_group_id ON public.group_requests USING btree (group_id);


--
-- Name: index_group_requests_on_group_id_and_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_group_requests_on_group_id_and_user_id ON public.group_requests USING btree (group_id, user_id);


--
-- Name: index_group_requests_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_group_requests_on_user_id ON public.group_requests USING btree (user_id);


--
-- Name: index_group_users_on_group_id_and_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_group_users_on_group_id_and_user_id ON public.group_users USING btree (group_id, user_id);


--
-- Name: index_group_users_on_user_id_and_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_group_users_on_user_id_and_group_id ON public.group_users USING btree (user_id, group_id);


--
-- Name: index_groups_on_incoming_email; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_groups_on_incoming_email ON public.groups USING btree (incoming_email);


--
-- Name: index_groups_on_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_groups_on_name ON public.groups USING btree (name);


--
-- Name: index_groups_web_hooks_on_web_hook_id_and_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_groups_web_hooks_on_web_hook_id_and_group_id ON public.groups_web_hooks USING btree (web_hook_id, group_id);


--
-- Name: index_house_ads_categories; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_house_ads_categories ON public.ad_plugin_house_ads_categories USING btree (ad_plugin_house_ad_id, category_id);


--
-- Name: index_house_ads_groups; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_house_ads_groups ON public.ad_plugin_house_ads_groups USING btree (ad_plugin_house_ad_id, group_id);


--
-- Name: index_house_ads_pages; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_house_ads_pages ON public.ad_plugin_house_ads_routes USING btree (ad_plugin_house_ad_id, route_name);


--
-- Name: index_ignored_users_on_ignored_user_id_and_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_ignored_users_on_ignored_user_id_and_user_id ON public.ignored_users USING btree (ignored_user_id, user_id);


--
-- Name: index_ignored_users_on_user_id_and_ignored_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_ignored_users_on_user_id_and_ignored_user_id ON public.ignored_users USING btree (user_id, ignored_user_id);


--
-- Name: index_incoming_chat_webhooks_on_key_and_chat_channel_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_incoming_chat_webhooks_on_key_and_chat_channel_id ON public.incoming_chat_webhooks USING btree (key, chat_channel_id);


--
-- Name: index_incoming_domains_on_name_and_https_and_port; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_incoming_domains_on_name_and_https_and_port ON public.incoming_domains USING btree (name, https, port);


--
-- Name: index_incoming_emails_on_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_incoming_emails_on_created_at ON public.incoming_emails USING btree (created_at);


--
-- Name: index_incoming_emails_on_error; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_incoming_emails_on_error ON public.incoming_emails USING btree (error);


--
-- Name: index_incoming_emails_on_imap_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_incoming_emails_on_imap_group_id ON public.incoming_emails USING btree (imap_group_id);


--
-- Name: index_incoming_emails_on_imap_sync; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_incoming_emails_on_imap_sync ON public.incoming_emails USING btree (imap_sync);


--
-- Name: index_incoming_emails_on_message_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_incoming_emails_on_message_id ON public.incoming_emails USING btree (message_id);


--
-- Name: index_incoming_emails_on_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_incoming_emails_on_post_id ON public.incoming_emails USING btree (post_id);


--
-- Name: index_incoming_emails_on_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_incoming_emails_on_topic_id ON public.incoming_emails USING btree (topic_id);


--
-- Name: index_incoming_emails_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_incoming_emails_on_user_id ON public.incoming_emails USING btree (user_id) WHERE (user_id IS NOT NULL);


--
-- Name: index_incoming_links_on_created_at_and_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_incoming_links_on_created_at_and_user_id ON public.incoming_links USING btree (created_at, user_id);


--
-- Name: index_incoming_links_on_current_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_incoming_links_on_current_user_id ON public.incoming_links USING btree (current_user_id) WHERE (current_user_id IS NOT NULL);


--
-- Name: index_incoming_links_on_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_incoming_links_on_post_id ON public.incoming_links USING btree (post_id);


--
-- Name: index_incoming_links_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_incoming_links_on_user_id ON public.incoming_links USING btree (user_id) WHERE (user_id IS NOT NULL);


--
-- Name: index_incoming_referers_on_path_and_incoming_domain_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_incoming_referers_on_path_and_incoming_domain_id ON public.incoming_referers USING btree (path, incoming_domain_id);


--
-- Name: index_inferred_concept_posts_on_inferred_concept_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_inferred_concept_posts_on_inferred_concept_id ON public.inferred_concept_posts USING btree (inferred_concept_id);


--
-- Name: index_inferred_concept_posts_uniqueness; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_inferred_concept_posts_uniqueness ON public.inferred_concept_posts USING btree (post_id, inferred_concept_id);


--
-- Name: index_inferred_concept_topics_on_inferred_concept_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_inferred_concept_topics_on_inferred_concept_id ON public.inferred_concept_topics USING btree (inferred_concept_id);


--
-- Name: index_inferred_concept_topics_uniqueness; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_inferred_concept_topics_uniqueness ON public.inferred_concept_topics USING btree (topic_id, inferred_concept_id);


--
-- Name: index_inferred_concepts_on_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_inferred_concepts_on_name ON public.inferred_concepts USING btree (name);


--
-- Name: index_invited_groups_on_group_id_and_invite_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_invited_groups_on_group_id_and_invite_id ON public.invited_groups USING btree (group_id, invite_id);


--
-- Name: index_invited_users_on_invite_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_invited_users_on_invite_id ON public.invited_users USING btree (invite_id);


--
-- Name: index_invited_users_on_user_id_and_invite_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_invited_users_on_user_id_and_invite_id ON public.invited_users USING btree (user_id, invite_id) WHERE (user_id IS NOT NULL);


--
-- Name: index_invites_on_email_and_invited_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_invites_on_email_and_invited_by_id ON public.invites USING btree (email, invited_by_id);


--
-- Name: index_invites_on_emailed_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_invites_on_emailed_status ON public.invites USING btree (emailed_status);


--
-- Name: index_invites_on_invite_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_invites_on_invite_key ON public.invites USING btree (invite_key);


--
-- Name: index_invites_on_invited_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_invites_on_invited_by_id ON public.invites USING btree (invited_by_id);


--
-- Name: index_javascript_caches_on_digest; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_javascript_caches_on_digest ON public.javascript_caches USING btree (digest);


--
-- Name: index_javascript_caches_on_theme_field_id_and_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_javascript_caches_on_theme_field_id_and_name ON public.javascript_caches USING btree (theme_field_id, name) NULLS NOT DISTINCT WHERE (theme_field_id IS NOT NULL);


--
-- Name: index_javascript_caches_on_theme_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_javascript_caches_on_theme_id ON public.javascript_caches USING btree (theme_id);


--
-- Name: index_linked_topics_on_topic_id_and_original_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_linked_topics_on_topic_id_and_original_topic_id ON public.linked_topics USING btree (topic_id, original_topic_id);


--
-- Name: index_linked_topics_on_topic_id_and_sequence; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_linked_topics_on_topic_id_and_sequence ON public.linked_topics USING btree (topic_id, sequence);


--
-- Name: index_llm_credit_allocations_on_llm_model_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_llm_credit_allocations_on_llm_model_id ON public.llm_credit_allocations USING btree (llm_model_id);


--
-- Name: index_llm_credit_daily_usages_on_llm_model_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_llm_credit_daily_usages_on_llm_model_id ON public.llm_credit_daily_usages USING btree (llm_model_id);


--
-- Name: index_llm_credit_daily_usages_on_llm_model_id_and_usage_date; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_llm_credit_daily_usages_on_llm_model_id_and_usage_date ON public.llm_credit_daily_usages USING btree (llm_model_id, usage_date);


--
-- Name: index_llm_feature_credit_costs_on_llm_model_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_llm_feature_credit_costs_on_llm_model_id ON public.llm_feature_credit_costs USING btree (llm_model_id);


--
-- Name: index_llm_models_on_ai_secret_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_llm_models_on_ai_secret_id ON public.llm_models USING btree (ai_secret_id);


--
-- Name: index_llm_models_on_vision_llm_model_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_llm_models_on_vision_llm_model_id ON public.llm_models USING btree (vision_llm_model_id);


--
-- Name: index_llm_quota_usages_on_llm_quota_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_llm_quota_usages_on_llm_quota_id ON public.llm_quota_usages USING btree (llm_quota_id);


--
-- Name: index_llm_quota_usages_on_user_id_and_llm_quota_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_llm_quota_usages_on_user_id_and_llm_quota_id ON public.llm_quota_usages USING btree (user_id, llm_quota_id);


--
-- Name: index_llm_quotas_on_group_id_and_llm_model_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_llm_quotas_on_group_id_and_llm_model_id ON public.llm_quotas USING btree (group_id, llm_model_id);


--
-- Name: index_llm_quotas_on_llm_model_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_llm_quotas_on_llm_model_id ON public.llm_quotas USING btree (llm_model_id);


--
-- Name: index_message_bus_on_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_message_bus_on_created_at ON public.message_bus USING btree (created_at);


--
-- Name: index_model_accuracies_on_model; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_model_accuracies_on_model ON public.model_accuracies USING btree (model);


--
-- Name: index_moved_posts_on_new_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_moved_posts_on_new_post_id ON public.moved_posts USING btree (new_post_id);


--
-- Name: index_moved_posts_on_new_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_moved_posts_on_new_topic_id ON public.moved_posts USING btree (new_topic_id);


--
-- Name: index_moved_posts_on_new_topic_id_and_post_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_moved_posts_on_new_topic_id_and_post_user_id ON public.moved_posts USING btree (new_topic_id, post_user_id);


--
-- Name: index_moved_posts_on_old_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_moved_posts_on_old_post_id ON public.moved_posts USING btree (old_post_id);


--
-- Name: index_moved_posts_on_old_post_number; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_moved_posts_on_old_post_number ON public.moved_posts USING btree (old_post_number);


--
-- Name: index_moved_posts_on_old_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_moved_posts_on_old_topic_id ON public.moved_posts USING btree (old_topic_id);


--
-- Name: index_muted_users_on_muted_user_id_and_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_muted_users_on_muted_user_id_and_user_id ON public.muted_users USING btree (muted_user_id, user_id);


--
-- Name: index_muted_users_on_user_id_and_muted_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_muted_users_on_user_id_and_muted_user_id ON public.muted_users USING btree (user_id, muted_user_id);


--
-- Name: index_nested_hot_post_scores_on_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_nested_hot_post_scores_on_post_id ON public.nested_hot_post_scores USING btree (post_id);


--
-- Name: index_nested_hot_post_scores_on_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_nested_hot_post_scores_on_topic_id ON public.nested_hot_post_scores USING btree (topic_id);


--
-- Name: index_nested_hot_score_snapshots_on_calculated_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_nested_hot_score_snapshots_on_calculated_at ON public.nested_hot_score_snapshots USING btree (calculated_at);


--
-- Name: index_nested_hot_score_snapshots_on_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_nested_hot_score_snapshots_on_topic_id ON public.nested_hot_score_snapshots USING btree (topic_id);


--
-- Name: index_nested_topics_on_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_nested_topics_on_topic_id ON public.nested_topics USING btree (topic_id);


--
-- Name: index_nested_view_post_stats_on_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_nested_view_post_stats_on_post_id ON public.nested_view_post_stats USING btree (post_id);


--
-- Name: index_notifications_on_data_display_username; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notifications_on_data_display_username ON public.notifications USING btree ((((data)::jsonb ->> 'display_username'::text))) WHERE (((data)::jsonb ->> 'display_username'::text) IS NOT NULL);


--
-- Name: index_notifications_on_data_original_username; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notifications_on_data_original_username ON public.notifications USING btree ((((data)::jsonb ->> 'original_username'::text))) WHERE (((data)::jsonb ->> 'original_username'::text) IS NOT NULL);


--
-- Name: index_notifications_on_data_username; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notifications_on_data_username ON public.notifications USING btree ((((data)::jsonb ->> 'username'::text))) WHERE (((data)::jsonb ->> 'username'::text) IS NOT NULL);


--
-- Name: index_notifications_on_data_username2; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notifications_on_data_username2 ON public.notifications USING btree ((((data)::jsonb ->> 'username2'::text))) WHERE (((data)::jsonb ->> 'username2'::text) IS NOT NULL);


--
-- Name: index_notifications_on_post_action_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notifications_on_post_action_id ON public.notifications USING btree (post_action_id);


--
-- Name: index_notifications_on_topic_id_and_post_number; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notifications_on_topic_id_and_post_number ON public.notifications USING btree (topic_id, post_number);


--
-- Name: index_notifications_on_user_id_and_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notifications_on_user_id_and_created_at ON public.notifications USING btree (user_id, created_at);


--
-- Name: index_notifications_on_user_id_and_topic_id_and_post_number; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notifications_on_user_id_and_topic_id_and_post_number ON public.notifications USING btree (user_id, topic_id, post_number);


--
-- Name: index_notifications_read_or_not_high_priority; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notifications_read_or_not_high_priority ON public.notifications USING btree (user_id, id DESC, read, topic_id) WHERE (read OR (high_priority = false));


--
-- Name: index_notifications_unique_unread_high_priority; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_notifications_unique_unread_high_priority ON public.notifications USING btree (user_id, id) WHERE ((NOT read) AND (high_priority = true));


--
-- Name: index_notifications_user_menu_ordering; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notifications_user_menu_ordering ON public.notifications USING btree (user_id, ((high_priority AND (NOT read))) DESC, ((NOT read)) DESC, created_at DESC);


--
-- Name: index_notifications_user_menu_ordering_deprioritized_likes; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notifications_user_menu_ordering_deprioritized_likes ON public.notifications USING btree (user_id, ((high_priority AND (NOT read))) DESC, (((NOT read) AND (notification_type <> ALL (ARRAY[5, 19, 25])))) DESC, created_at DESC);


--
-- Name: index_oauth2_user_infos_on_uid_and_provider; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_oauth2_user_infos_on_uid_and_provider ON public.oauth2_user_infos USING btree (uid, provider);


--
-- Name: index_oauth2_user_infos_on_user_id_and_provider; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_oauth2_user_infos_on_user_id_and_provider ON public.oauth2_user_infos USING btree (user_id, provider);


--
-- Name: index_onceoff_logs_on_job_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_onceoff_logs_on_job_name ON public.onceoff_logs USING btree (job_name);


--
-- Name: index_optimized_images_on_etag; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_optimized_images_on_etag ON public.optimized_images USING btree (etag);


--
-- Name: index_optimized_images_on_upload_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_optimized_images_on_upload_id ON public.optimized_images USING btree (upload_id);


--
-- Name: index_optimized_images_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_optimized_images_unique ON public.optimized_images USING btree (upload_id, width, height, extension);


--
-- Name: index_optimized_videos_on_optimized_upload_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_optimized_videos_on_optimized_upload_id ON public.optimized_videos USING btree (optimized_upload_id);


--
-- Name: index_optimized_videos_on_upload_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_optimized_videos_on_upload_id ON public.optimized_videos USING btree (upload_id);


--
-- Name: index_optimized_videos_on_upload_id_and_adapter; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_optimized_videos_on_upload_id_and_adapter ON public.optimized_videos USING btree (upload_id, adapter);


--
-- Name: index_permalinks_on_url; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_permalinks_on_url ON public.permalinks USING btree (url);


--
-- Name: index_plugin_store_rows_on_plugin_name_and_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_plugin_store_rows_on_plugin_name_and_key ON public.plugin_store_rows USING btree (plugin_name, key);


--
-- Name: index_policy_users_on_post_policy_id_and_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_policy_users_on_post_policy_id_and_user_id ON public.policy_users USING btree (post_policy_id, user_id);


--
-- Name: index_poll_options_on_poll_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_poll_options_on_poll_id ON public.poll_options USING btree (poll_id);


--
-- Name: index_poll_options_on_poll_id_and_digest; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_poll_options_on_poll_id_and_digest ON public.poll_options USING btree (poll_id, digest);


--
-- Name: index_poll_votes_on_poll_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_poll_votes_on_poll_id ON public.poll_votes USING btree (poll_id);


--
-- Name: index_poll_votes_on_poll_id_and_poll_option_id_and_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_poll_votes_on_poll_id_and_poll_option_id_and_user_id ON public.poll_votes USING btree (poll_id, poll_option_id, user_id);


--
-- Name: index_poll_votes_on_poll_option_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_poll_votes_on_poll_option_id ON public.poll_votes USING btree (poll_option_id);


--
-- Name: index_poll_votes_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_poll_votes_on_user_id ON public.poll_votes USING btree (user_id);


--
-- Name: index_polls_on_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_polls_on_post_id ON public.polls USING btree (post_id);


--
-- Name: index_polls_on_post_id_and_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_polls_on_post_id_and_name ON public.polls USING btree (post_id, name);


--
-- Name: index_post_actions_on_agreed_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_actions_on_agreed_by_id ON public.post_actions USING btree (agreed_by_id) WHERE (agreed_by_id IS NOT NULL);


--
-- Name: index_post_actions_on_deferred_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_actions_on_deferred_by_id ON public.post_actions USING btree (deferred_by_id) WHERE (deferred_by_id IS NOT NULL);


--
-- Name: index_post_actions_on_deleted_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_actions_on_deleted_by_id ON public.post_actions USING btree (deleted_by_id) WHERE (deleted_by_id IS NOT NULL);


--
-- Name: index_post_actions_on_disagreed_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_actions_on_disagreed_by_id ON public.post_actions USING btree (disagreed_by_id) WHERE (disagreed_by_id IS NOT NULL);


--
-- Name: index_post_actions_on_post_action_type_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_actions_on_post_action_type_id ON public.post_actions USING btree (post_action_type_id);


--
-- Name: index_post_actions_on_post_action_type_id_and_disagreed_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_actions_on_post_action_type_id_and_disagreed_at ON public.post_actions USING btree (post_action_type_id, disagreed_at) WHERE (disagreed_at IS NULL);


--
-- Name: index_post_actions_on_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_actions_on_post_id ON public.post_actions USING btree (post_id);


--
-- Name: index_post_actions_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_actions_on_user_id ON public.post_actions USING btree (user_id);


--
-- Name: index_post_actions_on_user_id_and_post_action_type_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_actions_on_user_id_and_post_action_type_id ON public.post_actions USING btree (user_id, post_action_type_id) WHERE (deleted_at IS NULL);


--
-- Name: index_post_custom_fields_on_name_and_value; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_custom_fields_on_name_and_value ON public.post_custom_fields USING btree (name, "left"(value, 200));


--
-- Name: index_post_custom_fields_on_notice; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_post_custom_fields_on_notice ON public.post_custom_fields USING btree (post_id) WHERE ((name)::text = 'notice'::text);


--
-- Name: index_post_custom_fields_on_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_post_custom_fields_on_post_id ON public.post_custom_fields USING btree (post_id) WHERE ((name)::text = 'missing uploads'::text);


--
-- Name: index_post_custom_fields_on_post_id_and_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_custom_fields_on_post_id_and_name ON public.post_custom_fields USING btree (post_id, name);


--
-- Name: index_post_custom_fields_on_stalled_wiki_triggered_at; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_post_custom_fields_on_stalled_wiki_triggered_at ON public.post_custom_fields USING btree (post_id) WHERE ((name)::text = 'stalled_wiki_triggered_at'::text);


--
-- Name: index_post_custom_prompts_on_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_post_custom_prompts_on_post_id ON public.post_custom_prompts USING btree (post_id);


--
-- Name: index_post_details_on_post_id_and_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_post_details_on_post_id_and_key ON public.post_details USING btree (post_id, key);


--
-- Name: index_post_hotlinked_media_on_post_id_and_url_md5; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_post_hotlinked_media_on_post_id_and_url_md5 ON public.post_hotlinked_media USING btree (post_id, md5((url)::text));


--
-- Name: index_post_id_where_missing_uploads_ignored; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_post_id_where_missing_uploads_ignored ON public.post_custom_fields USING btree (post_id) WHERE ((name)::text = 'missing uploads ignored'::text);


--
-- Name: index_post_localizations_on_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_localizations_on_post_id ON public.post_localizations USING btree (post_id);


--
-- Name: index_post_localizations_on_post_id_and_locale; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_post_localizations_on_post_id_and_locale ON public.post_localizations USING btree (post_id, locale);


--
-- Name: index_post_policy_groups_on_post_policy_id_and_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_post_policy_groups_on_post_policy_id_and_group_id ON public.post_policy_groups USING btree (post_policy_id, group_id);


--
-- Name: index_post_replies_on_post_id_and_reply_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_post_replies_on_post_id_and_reply_post_id ON public.post_replies USING btree (post_id, reply_post_id);


--
-- Name: index_post_replies_on_reply_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_replies_on_reply_post_id ON public.post_replies USING btree (reply_post_id);


--
-- Name: index_post_reply_keys_on_reply_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_post_reply_keys_on_reply_key ON public.post_reply_keys USING btree (reply_key);


--
-- Name: index_post_reply_keys_on_user_id_and_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_post_reply_keys_on_user_id_and_post_id ON public.post_reply_keys USING btree (user_id, post_id);


--
-- Name: index_post_revisions_on_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_revisions_on_post_id ON public.post_revisions USING btree (post_id);


--
-- Name: index_post_revisions_on_post_id_and_number; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_revisions_on_post_id_and_number ON public.post_revisions USING btree (post_id, number);


--
-- Name: index_post_revisions_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_revisions_on_user_id ON public.post_revisions USING btree (user_id);


--
-- Name: index_post_search_data_on_post_id_and_version_and_locale; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_search_data_on_post_id_and_version_and_locale ON public.post_search_data USING btree (post_id, version, locale);


--
-- Name: index_post_stats_on_composer_version; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_stats_on_composer_version ON public.post_stats USING btree (composer_version);


--
-- Name: index_post_stats_on_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_stats_on_post_id ON public.post_stats USING btree (post_id);


--
-- Name: index_post_stats_on_writing_device; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_stats_on_writing_device ON public.post_stats USING btree (writing_device);


--
-- Name: index_post_timings_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_timings_on_user_id ON public.post_timings USING btree (user_id);


--
-- Name: index_post_voting_comments_on_deleted_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_voting_comments_on_deleted_by_id ON public.post_voting_comments USING btree (deleted_by_id) WHERE (deleted_by_id IS NOT NULL);


--
-- Name: index_post_voting_comments_on_last_editor_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_voting_comments_on_last_editor_id ON public.post_voting_comments USING btree (last_editor_id);


--
-- Name: index_post_voting_comments_on_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_voting_comments_on_post_id ON public.post_voting_comments USING btree (post_id);


--
-- Name: index_post_voting_comments_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_post_voting_comments_on_user_id ON public.post_voting_comments USING btree (user_id);


--
-- Name: index_posts_on_deleted_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_posts_on_deleted_by_id ON public.posts USING btree (deleted_by_id) WHERE (deleted_by_id IS NOT NULL);


--
-- Name: index_posts_on_id_and_baked_version; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_posts_on_id_and_baked_version ON public.posts USING btree (id DESC, baked_version) WHERE (deleted_at IS NULL);


--
-- Name: index_posts_on_id_topic_id_where_not_deleted_or_empty; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_posts_on_id_topic_id_where_not_deleted_or_empty ON public.posts USING btree (id, topic_id) WHERE ((deleted_at IS NULL) AND (raw <> ''::text));


--
-- Name: index_posts_on_image_upload_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_posts_on_image_upload_id ON public.posts USING btree (image_upload_id);


--
-- Name: index_posts_on_last_editor_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_posts_on_last_editor_id ON public.posts USING btree (last_editor_id) WHERE (last_editor_id IS NOT NULL);


--
-- Name: index_posts_on_locale; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_posts_on_locale ON public.posts USING btree (locale);


--
-- Name: index_posts_on_locked_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_posts_on_locked_by_id ON public.posts USING btree (locked_by_id) WHERE (locked_by_id IS NOT NULL);


--
-- Name: index_posts_on_reply_to_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_posts_on_reply_to_user_id ON public.posts USING btree (reply_to_user_id) WHERE (reply_to_user_id IS NOT NULL);


--
-- Name: index_posts_on_topic_id_and_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_posts_on_topic_id_and_created_at ON public.posts USING btree (topic_id, created_at);


--
-- Name: index_posts_on_topic_id_and_percent_rank; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_posts_on_topic_id_and_percent_rank ON public.posts USING btree (topic_id, percent_rank);


--
-- Name: index_posts_on_topic_id_and_post_number; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_posts_on_topic_id_and_post_number ON public.posts USING btree (topic_id, post_number);


--
-- Name: index_posts_on_topic_id_and_reply_to_post_number; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_posts_on_topic_id_and_reply_to_post_number ON public.posts USING btree (topic_id, reply_to_post_number);


--
-- Name: index_posts_on_topic_id_and_sort_order; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_posts_on_topic_id_and_sort_order ON public.posts USING btree (topic_id, sort_order);


--
-- Name: index_posts_on_updated_at_for_locale_detection; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_posts_on_updated_at_for_locale_detection ON public.posts USING btree (updated_at DESC) WHERE ((deleted_at IS NULL) AND (user_id > 0) AND (locale IS NULL));


--
-- Name: index_posts_on_updated_at_for_localization; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_posts_on_updated_at_for_localization ON public.posts USING btree (updated_at DESC) WHERE ((deleted_at IS NULL) AND (user_id > 0) AND (locale IS NOT NULL));


--
-- Name: index_posts_on_user_id_and_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_posts_on_user_id_and_created_at ON public.posts USING btree (user_id, created_at);


--
-- Name: index_posts_user_and_likes; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_posts_user_and_likes ON public.posts USING btree (user_id, like_count DESC, created_at DESC) WHERE (post_number > 1);


--
-- Name: index_problem_check_trackers_on_identifier_and_target; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_problem_check_trackers_on_identifier_and_target ON public.problem_check_trackers USING btree (identifier, target);


--
-- Name: index_published_pages_on_slug; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_published_pages_on_slug ON public.published_pages USING btree (slug);


--
-- Name: index_published_pages_on_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_published_pages_on_topic_id ON public.published_pages USING btree (topic_id);


--
-- Name: index_quoted_posts_on_post_id_and_quoted_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_quoted_posts_on_post_id_and_quoted_post_id ON public.quoted_posts USING btree (post_id, quoted_post_id);


--
-- Name: index_quoted_posts_on_quoted_post_id_and_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_quoted_posts_on_quoted_post_id_and_post_id ON public.quoted_posts USING btree (quoted_post_id, post_id);


--
-- Name: index_rag_document_fragments_on_target_type_and_target_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_rag_document_fragments_on_target_type_and_target_id ON public.rag_document_fragments USING btree (target_type, target_id);


--
-- Name: index_rag_document_sources_on_next_refresh_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_rag_document_sources_on_next_refresh_at ON public.rag_document_sources USING btree (next_refresh_at);


--
-- Name: index_rag_document_sources_on_target_type_and_target_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_rag_document_sources_on_target_type_and_target_id ON public.rag_document_sources USING btree (target_type, target_id);


--
-- Name: index_redelivering_webhook_events_on_web_hook_event_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_redelivering_webhook_events_on_web_hook_event_id ON public.redelivering_webhook_events USING btree (web_hook_event_id);


--
-- Name: index_reviewable_claimed_topics_on_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_reviewable_claimed_topics_on_topic_id ON public.reviewable_claimed_topics USING btree (topic_id);


--
-- Name: index_reviewable_histories_on_created_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_reviewable_histories_on_created_by_id ON public.reviewable_histories USING btree (created_by_id);


--
-- Name: index_reviewable_histories_on_reviewable_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_reviewable_histories_on_reviewable_id ON public.reviewable_histories USING btree (reviewable_id);


--
-- Name: index_reviewable_notes_on_reviewable_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_reviewable_notes_on_reviewable_id ON public.reviewable_notes USING btree (reviewable_id);


--
-- Name: index_reviewable_notes_on_reviewable_id_and_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_reviewable_notes_on_reviewable_id_and_created_at ON public.reviewable_notes USING btree (reviewable_id, created_at);


--
-- Name: index_reviewable_notes_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_reviewable_notes_on_user_id ON public.reviewable_notes USING btree (user_id);


--
-- Name: index_reviewable_scores_on_reviewable_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_reviewable_scores_on_reviewable_id ON public.reviewable_scores USING btree (reviewable_id);


--
-- Name: index_reviewable_scores_on_reviewable_score_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_reviewable_scores_on_reviewable_score_type ON public.reviewable_scores USING btree (reviewable_score_type);


--
-- Name: index_reviewable_scores_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_reviewable_scores_on_user_id ON public.reviewable_scores USING btree (user_id);


--
-- Name: index_reviewables_on_reviewable_by_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_reviewables_on_reviewable_by_group_id ON public.reviewables USING btree (reviewable_by_group_id);


--
-- Name: index_reviewables_on_status_and_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_reviewables_on_status_and_created_at ON public.reviewables USING btree (status, created_at);


--
-- Name: index_reviewables_on_status_and_score; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_reviewables_on_status_and_score ON public.reviewables USING btree (status, score);


--
-- Name: index_reviewables_on_status_and_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_reviewables_on_status_and_type ON public.reviewables USING btree (status, type);


--
-- Name: index_reviewables_on_target_created_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_reviewables_on_target_created_by_id ON public.reviewables USING btree (target_created_by_id);


--
-- Name: index_reviewables_on_target_id_where_post_type_eq_post; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_reviewables_on_target_id_where_post_type_eq_post ON public.reviewables USING btree (target_id) WHERE ((target_type)::text = 'Post'::text);


--
-- Name: index_reviewables_on_topic_id_and_status_and_created_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_reviewables_on_topic_id_and_status_and_created_by_id ON public.reviewables USING btree (topic_id, status, created_by_id);


--
-- Name: index_reviewables_on_type_and_target_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_reviewables_on_type_and_target_id ON public.reviewables USING btree (type, target_id);


--
-- Name: index_schema_migration_details_on_version; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_schema_migration_details_on_version ON public.schema_migration_details USING btree (version);


--
-- Name: index_screened_emails_on_email; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_screened_emails_on_email ON public.screened_emails USING btree (email);


--
-- Name: index_screened_emails_on_last_match_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_screened_emails_on_last_match_at ON public.screened_emails USING btree (last_match_at);


--
-- Name: index_screened_ip_addresses_on_ip_address; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_screened_ip_addresses_on_ip_address ON public.screened_ip_addresses USING btree (ip_address);


--
-- Name: index_screened_ip_addresses_on_last_match_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_screened_ip_addresses_on_last_match_at ON public.screened_ip_addresses USING btree (last_match_at);


--
-- Name: index_screened_urls_on_last_match_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_screened_urls_on_last_match_at ON public.screened_urls USING btree (last_match_at);


--
-- Name: index_screened_urls_on_url; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_screened_urls_on_url ON public.screened_urls USING btree (url);


--
-- Name: index_search_logs_on_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_search_logs_on_created_at ON public.search_logs USING btree (created_at);


--
-- Name: index_search_logs_on_created_at_excluding_crawlers; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_search_logs_on_created_at_excluding_crawlers ON public.search_logs USING btree (created_at) WHERE ((NOT crawler) AND (NOT likely_crawler));


--
-- Name: index_search_logs_on_user_id_and_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_search_logs_on_user_id_and_created_at ON public.search_logs USING btree (user_id, created_at) WHERE (user_id IS NOT NULL);


--
-- Name: index_shared_ai_conversations_on_share_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_shared_ai_conversations_on_share_key ON public.shared_ai_conversations USING btree (share_key);


--
-- Name: index_shared_ai_conversations_on_target_id_and_target_type; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_shared_ai_conversations_on_target_id_and_target_type ON public.shared_ai_conversations USING btree (target_id, target_type);


--
-- Name: index_shared_drafts_on_category_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_shared_drafts_on_category_id ON public.shared_drafts USING btree (category_id);


--
-- Name: index_shared_drafts_on_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_shared_drafts_on_topic_id ON public.shared_drafts USING btree (topic_id);


--
-- Name: index_shelved_notifications_on_notification_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_shelved_notifications_on_notification_id ON public.shelved_notifications USING btree (notification_id);


--
-- Name: index_sidebar_section_links_on_linkable_type_and_linkable_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_sidebar_section_links_on_linkable_type_and_linkable_id ON public.sidebar_section_links USING btree (linkable_type, linkable_id);


--
-- Name: index_sidebar_section_localizations_on_sidebar_section_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_sidebar_section_localizations_on_sidebar_section_id ON public.sidebar_section_localizations USING btree (sidebar_section_id);


--
-- Name: index_sidebar_sections_on_section_type; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_sidebar_sections_on_section_type ON public.sidebar_sections USING btree (section_type);


--
-- Name: index_sidebar_sections_on_user_id_and_title; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_sidebar_sections_on_user_id_and_title ON public.sidebar_sections USING btree (user_id, title);


--
-- Name: index_sidebar_url_localizations_on_sidebar_url_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_sidebar_url_localizations_on_sidebar_url_id ON public.sidebar_url_localizations USING btree (sidebar_url_id);


--
-- Name: index_sidebar_url_localizations_on_sidebar_url_id_and_locale; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_sidebar_url_localizations_on_sidebar_url_id_and_locale ON public.sidebar_url_localizations USING btree (sidebar_url_id, locale);


--
-- Name: index_silenced_assignments_on_assignment_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_silenced_assignments_on_assignment_id ON public.silenced_assignments USING btree (assignment_id);


--
-- Name: index_single_sign_on_records_on_external_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_single_sign_on_records_on_external_id ON public.single_sign_on_records USING btree (external_id);


--
-- Name: index_single_sign_on_records_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_single_sign_on_records_on_user_id ON public.single_sign_on_records USING btree (user_id);


--
-- Name: index_site_setting_groups_on_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_site_setting_groups_on_name ON public.site_setting_groups USING btree (name);


--
-- Name: index_site_setting_localizations_on_locale; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_site_setting_localizations_on_locale ON public.site_setting_localizations USING btree (locale);


--
-- Name: index_site_setting_localizations_on_setting_name_and_locale; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_site_setting_localizations_on_setting_name_and_locale ON public.site_setting_localizations USING btree (setting_name, locale);


--
-- Name: index_site_settings_on_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_site_settings_on_name ON public.site_settings USING btree (name);


--
-- Name: index_sitemaps_on_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_sitemaps_on_name ON public.sitemaps USING btree (name);


--
-- Name: index_skipped_email_logs_on_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_skipped_email_logs_on_created_at ON public.skipped_email_logs USING btree (created_at);


--
-- Name: index_skipped_email_logs_on_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_skipped_email_logs_on_post_id ON public.skipped_email_logs USING btree (post_id);


--
-- Name: index_skipped_email_logs_on_reason_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_skipped_email_logs_on_reason_type ON public.skipped_email_logs USING btree (reason_type);


--
-- Name: index_skipped_email_logs_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_skipped_email_logs_on_user_id ON public.skipped_email_logs USING btree (user_id);


--
-- Name: index_stylesheet_cache_on_target_and_digest; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_stylesheet_cache_on_target_and_digest ON public.stylesheet_cache USING btree (target, digest);


--
-- Name: index_tag_group_memberships_on_tag_group_id_and_tag_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_tag_group_memberships_on_tag_group_id_and_tag_id ON public.tag_group_memberships USING btree (tag_group_id, tag_id);


--
-- Name: index_tag_group_permissions_on_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_tag_group_permissions_on_group_id ON public.tag_group_permissions USING btree (group_id);


--
-- Name: index_tag_group_permissions_on_tag_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_tag_group_permissions_on_tag_group_id ON public.tag_group_permissions USING btree (tag_group_id);


--
-- Name: index_tag_groups_on_lower_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_tag_groups_on_lower_name ON public.tag_groups USING btree (lower((name)::text));


--
-- Name: index_tag_localizations_on_description_cooked_version; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_tag_localizations_on_description_cooked_version ON public.tag_localizations USING btree (description_cooked_version);


--
-- Name: index_tag_localizations_on_tag_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_tag_localizations_on_tag_id ON public.tag_localizations USING btree (tag_id);


--
-- Name: index_tag_localizations_on_tag_id_and_locale; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_tag_localizations_on_tag_id_and_locale ON public.tag_localizations USING btree (tag_id, locale);


--
-- Name: index_tag_users_on_tag_id_and_user_id_and_notification_level; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_tag_users_on_tag_id_and_user_id_and_notification_level ON public.tag_users USING btree (tag_id, user_id, notification_level);


--
-- Name: index_tag_users_on_user_id_and_tag_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_tag_users_on_user_id_and_tag_id ON public.tag_users USING btree (user_id, tag_id);


--
-- Name: index_tag_users_on_user_id_and_tag_id_and_notification_level; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_tag_users_on_user_id_and_tag_id_and_notification_level ON public.tag_users USING btree (user_id, tag_id, notification_level);


--
-- Name: index_tags_on_description_cooked_version; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_tags_on_description_cooked_version ON public.tags USING btree (description_cooked_version);


--
-- Name: index_tags_on_lower_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_tags_on_lower_name ON public.tags USING btree (lower((name)::text));


--
-- Name: index_tags_on_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_tags_on_name ON public.tags USING btree (name);


--
-- Name: index_tags_on_slug; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_tags_on_slug ON public.tags USING btree (slug) WHERE ((slug)::text <> ''::text);


--
-- Name: index_tags_on_target_tag_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_tags_on_target_tag_id ON public.tags USING btree (target_tag_id) WHERE (target_tag_id IS NOT NULL);


--
-- Name: index_theme_modifier_sets_on_theme_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_theme_modifier_sets_on_theme_id ON public.theme_modifier_sets USING btree (theme_id);


--
-- Name: index_theme_settings_migrations_on_theme_field_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_theme_settings_migrations_on_theme_field_id ON public.theme_settings_migrations USING btree (theme_field_id);


--
-- Name: index_theme_settings_migrations_on_theme_id_and_version; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_theme_settings_migrations_on_theme_id_and_version ON public.theme_settings_migrations USING btree (theme_id, version);


--
-- Name: index_theme_site_settings_on_theme_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_theme_site_settings_on_theme_id ON public.theme_site_settings USING btree (theme_id);


--
-- Name: index_theme_site_settings_on_theme_id_and_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_theme_site_settings_on_theme_id_and_name ON public.theme_site_settings USING btree (theme_id, name);


--
-- Name: index_theme_svg_sprites_on_theme_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_theme_svg_sprites_on_theme_id ON public.theme_svg_sprites USING btree (theme_id);


--
-- Name: index_theme_translation_overrides_on_theme_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_theme_translation_overrides_on_theme_id ON public.theme_translation_overrides USING btree (theme_id);


--
-- Name: index_themes_on_remote_theme_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_themes_on_remote_theme_id ON public.themes USING btree (remote_theme_id);


--
-- Name: index_top_topics_on_all_score; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_top_topics_on_all_score ON public.top_topics USING btree (all_score);


--
-- Name: index_top_topics_on_daily_score; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_top_topics_on_daily_score ON public.top_topics USING btree (daily_score);


--
-- Name: index_top_topics_on_monthly_score; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_top_topics_on_monthly_score ON public.top_topics USING btree (monthly_score);


--
-- Name: index_top_topics_on_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_top_topics_on_topic_id ON public.top_topics USING btree (topic_id);


--
-- Name: index_top_topics_on_weekly_score; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_top_topics_on_weekly_score ON public.top_topics USING btree (weekly_score);


--
-- Name: index_top_topics_on_yearly_score; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_top_topics_on_yearly_score ON public.top_topics USING btree (yearly_score);


--
-- Name: index_topic_allowed_groups_on_group_id_and_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_topic_allowed_groups_on_group_id_and_topic_id ON public.topic_allowed_groups USING btree (group_id, topic_id);


--
-- Name: index_topic_allowed_groups_on_topic_id_and_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_topic_allowed_groups_on_topic_id_and_group_id ON public.topic_allowed_groups USING btree (topic_id, group_id);


--
-- Name: index_topic_allowed_users_on_topic_id_and_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_topic_allowed_users_on_topic_id_and_user_id ON public.topic_allowed_users USING btree (topic_id, user_id);


--
-- Name: index_topic_allowed_users_on_user_id_and_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_topic_allowed_users_on_user_id_and_topic_id ON public.topic_allowed_users USING btree (user_id, topic_id);


--
-- Name: index_topic_custom_fields_on_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_topic_custom_fields_on_topic_id ON public.topic_custom_fields USING btree (topic_id) WHERE ((name)::text = 'vote_count'::text);


--
-- Name: index_topic_custom_fields_on_topic_id_and_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topic_custom_fields_on_topic_id_and_name ON public.topic_custom_fields USING btree (topic_id, name);


--
-- Name: index_topic_custom_fields_on_topic_id_and_slack_thread_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_topic_custom_fields_on_topic_id_and_slack_thread_id ON public.topic_custom_fields USING btree (topic_id, name) WHERE ((name)::text ~~ 'slack_thread_id_%'::text);


--
-- Name: index_topic_embeds_on_embed_url; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_topic_embeds_on_embed_url ON public.topic_embeds USING btree (embed_url);


--
-- Name: index_topic_groups_on_group_id_and_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_topic_groups_on_group_id_and_topic_id ON public.topic_groups USING btree (group_id, topic_id);


--
-- Name: index_topic_hot_scores_on_score_and_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_topic_hot_scores_on_score_and_topic_id ON public.topic_hot_scores USING btree (score, topic_id);


--
-- Name: index_topic_hot_scores_on_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_topic_hot_scores_on_topic_id ON public.topic_hot_scores USING btree (topic_id);


--
-- Name: index_topic_invites_on_invite_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topic_invites_on_invite_id ON public.topic_invites USING btree (invite_id);


--
-- Name: index_topic_invites_on_topic_id_and_invite_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_topic_invites_on_topic_id_and_invite_id ON public.topic_invites USING btree (topic_id, invite_id);


--
-- Name: index_topic_links_on_extension; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topic_links_on_extension ON public.topic_links USING btree (extension);


--
-- Name: index_topic_links_on_link_post_id_and_reflection; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topic_links_on_link_post_id_and_reflection ON public.topic_links USING btree (link_post_id, reflection);


--
-- Name: index_topic_links_on_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topic_links_on_post_id ON public.topic_links USING btree (post_id);


--
-- Name: index_topic_links_on_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topic_links_on_topic_id ON public.topic_links USING btree (topic_id);


--
-- Name: index_topic_links_on_user_and_clicks; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topic_links_on_user_and_clicks ON public.topic_links USING btree (user_id, clicks DESC, created_at DESC) WHERE ((NOT reflection) AND (NOT quote) AND (NOT internal));


--
-- Name: index_topic_links_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topic_links_on_user_id ON public.topic_links USING btree (user_id);


--
-- Name: index_topic_localizations_on_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topic_localizations_on_topic_id ON public.topic_localizations USING btree (topic_id);


--
-- Name: index_topic_localizations_on_topic_id_and_locale; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_topic_localizations_on_topic_id_and_locale ON public.topic_localizations USING btree (topic_id, locale);


--
-- Name: index_topic_search_data_on_topic_id_and_version_and_locale; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topic_search_data_on_topic_id_and_version_and_locale ON public.topic_search_data USING btree (topic_id, version, locale);


--
-- Name: index_topic_tags_on_tag_id_and_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_topic_tags_on_tag_id_and_topic_id ON public.topic_tags USING btree (tag_id, topic_id);


--
-- Name: index_topic_tags_on_topic_id_and_tag_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_topic_tags_on_topic_id_and_tag_id ON public.topic_tags USING btree (topic_id, tag_id);


--
-- Name: index_topic_thumbnails_on_optimized_image_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topic_thumbnails_on_optimized_image_id ON public.topic_thumbnails USING btree (optimized_image_id);


--
-- Name: index_topic_thumbnails_on_upload_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topic_thumbnails_on_upload_id ON public.topic_thumbnails USING btree (upload_id);


--
-- Name: index_topic_timers_on_timerable_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topic_timers_on_timerable_id ON public.topic_timers USING btree (timerable_id) WHERE (deleted_at IS NULL);


--
-- Name: index_topic_timers_on_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topic_timers_on_topic_id ON public.topic_timers USING btree (topic_id) WHERE (deleted_at IS NULL);


--
-- Name: index_topic_timers_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topic_timers_on_user_id ON public.topic_timers USING btree (user_id);


--
-- Name: index_topic_users_on_topic_id_and_notification_level; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topic_users_on_topic_id_and_notification_level ON public.topic_users USING btree (topic_id, notification_level);


--
-- Name: index_topic_users_on_topic_id_and_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_topic_users_on_topic_id_and_user_id ON public.topic_users USING btree (topic_id, user_id);


--
-- Name: index_topic_users_on_user_id_and_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_topic_users_on_user_id_and_topic_id ON public.topic_users USING btree (user_id, topic_id);


--
-- Name: index_topic_view_stats_on_topic_id_and_viewed_at; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_topic_view_stats_on_topic_id_and_viewed_at ON public.topic_view_stats USING btree (topic_id, viewed_at);


--
-- Name: index_topic_view_stats_on_viewed_at_and_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topic_view_stats_on_viewed_at_and_topic_id ON public.topic_view_stats USING btree (viewed_at, topic_id);


--
-- Name: index_topic_views_for_user_participation; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topic_views_for_user_participation ON public.topic_views USING btree (viewed_at, user_id, topic_id) WHERE (user_id IS NOT NULL);


--
-- Name: index_topic_views_on_topic_id_and_viewed_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topic_views_on_topic_id_and_viewed_at ON public.topic_views USING btree (topic_id, viewed_at);


--
-- Name: index_topic_views_on_user_id_and_viewed_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topic_views_on_user_id_and_viewed_at ON public.topic_views USING btree (user_id, viewed_at);


--
-- Name: index_topic_views_on_viewed_at_and_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topic_views_on_viewed_at_and_topic_id ON public.topic_views USING btree (viewed_at, topic_id);


--
-- Name: index_topic_voting_topic_vote_count_on_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_topic_voting_topic_vote_count_on_topic_id ON public.topic_voting_topic_vote_count USING btree (topic_id);


--
-- Name: index_topic_voting_topic_vote_count_on_votes_count; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topic_voting_topic_vote_count_on_votes_count ON public.topic_voting_topic_vote_count USING btree (votes_count);


--
-- Name: index_topic_voting_votes_on_topic_id_and_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topic_voting_votes_on_topic_id_and_created_at ON public.topic_voting_votes USING btree (topic_id, created_at);


--
-- Name: index_topics_on_bannered_until; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topics_on_bannered_until ON public.topics USING btree (bannered_until) WHERE (bannered_until IS NOT NULL);


--
-- Name: index_topics_on_bumped_at_public; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topics_on_bumped_at_public ON public.topics USING btree (bumped_at) WHERE ((deleted_at IS NULL) AND ((archetype)::text <> 'private_message'::text));


--
-- Name: index_topics_on_category_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topics_on_category_id ON public.topics USING btree (category_id) WHERE ((deleted_at IS NULL) AND ((archetype)::text <> 'private_message'::text));


--
-- Name: index_topics_on_created_at_and_visible; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topics_on_created_at_and_visible ON public.topics USING btree (created_at, visible) WHERE ((deleted_at IS NULL) AND ((archetype)::text <> 'private_message'::text));


--
-- Name: index_topics_on_external_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_topics_on_external_id ON public.topics USING btree (external_id) WHERE (external_id IS NOT NULL);


--
-- Name: index_topics_on_id_and_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topics_on_id_and_deleted_at ON public.topics USING btree (id, deleted_at);


--
-- Name: index_topics_on_id_filtered_banner; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_topics_on_id_filtered_banner ON public.topics USING btree (id) WHERE (((archetype)::text = 'banner'::text) AND (deleted_at IS NULL));


--
-- Name: index_topics_on_image_upload_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topics_on_image_upload_id ON public.topics USING btree (image_upload_id);


--
-- Name: index_topics_on_lower_title; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topics_on_lower_title ON public.topics USING btree (lower((title)::text));


--
-- Name: index_topics_on_pinned_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topics_on_pinned_at ON public.topics USING btree (pinned_at) WHERE (pinned_at IS NOT NULL);


--
-- Name: index_topics_on_pinned_globally; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topics_on_pinned_globally ON public.topics USING btree (pinned_globally) WHERE pinned_globally;


--
-- Name: index_topics_on_pinned_until; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topics_on_pinned_until ON public.topics USING btree (pinned_until) WHERE (pinned_until IS NOT NULL);


--
-- Name: index_topics_on_timestamps_private; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topics_on_timestamps_private ON public.topics USING btree (bumped_at, created_at, updated_at) WHERE ((deleted_at IS NULL) AND ((archetype)::text = 'private_message'::text));


--
-- Name: index_topics_on_updated_at_for_locale_detection; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topics_on_updated_at_for_locale_detection ON public.topics USING btree (updated_at DESC) WHERE ((deleted_at IS NULL) AND (user_id > 0) AND (locale IS NULL));


--
-- Name: index_topics_on_updated_at_for_localization; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topics_on_updated_at_for_localization ON public.topics USING btree (updated_at DESC) WHERE ((deleted_at IS NULL) AND (user_id > 0) AND (locale IS NOT NULL));


--
-- Name: index_topics_on_updated_at_public; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_topics_on_updated_at_public ON public.topics USING btree (updated_at, visible, highest_staff_post_number, highest_post_number, category_id, created_at, id) WHERE (((archetype)::text <> 'private_message'::text) AND (deleted_at IS NULL));


--
-- Name: index_translation_overrides_on_locale_and_translation_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_translation_overrides_on_locale_and_translation_key ON public.translation_overrides USING btree (locale, translation_key);


--
-- Name: index_unsubscribe_keys_on_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_unsubscribe_keys_on_created_at ON public.unsubscribe_keys USING btree (created_at);


--
-- Name: index_upcoming_change_events_on_event_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_upcoming_change_events_on_event_type ON public.upcoming_change_events USING btree (event_type);


--
-- Name: index_upcoming_change_events_on_upcoming_change_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_upcoming_change_events_on_upcoming_change_name ON public.upcoming_change_events USING btree (upcoming_change_name);


--
-- Name: index_upload_references_on_target; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_upload_references_on_target ON public.upload_references USING btree (target_type, target_id);


--
-- Name: index_upload_references_on_upload_and_target; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_upload_references_on_upload_and_target ON public.upload_references USING btree (upload_id, target_type, target_id);


--
-- Name: index_upload_references_on_upload_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_upload_references_on_upload_id ON public.upload_references USING btree (upload_id);


--
-- Name: index_uploads_on_access_control_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_uploads_on_access_control_post_id ON public.uploads USING btree (access_control_post_id);


--
-- Name: index_uploads_on_etag; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_uploads_on_etag ON public.uploads USING btree (etag);


--
-- Name: index_uploads_on_extension; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_uploads_on_extension ON public.uploads USING btree (lower((extension)::text));


--
-- Name: index_uploads_on_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_uploads_on_id ON public.uploads USING btree (id) WHERE (dominant_color IS NULL);


--
-- Name: index_uploads_on_id_and_url; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_uploads_on_id_and_url ON public.uploads USING btree (id, url);


--
-- Name: index_uploads_on_original_sha1; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_uploads_on_original_sha1 ON public.uploads USING btree (original_sha1);


--
-- Name: index_uploads_on_sha1; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_uploads_on_sha1 ON public.uploads USING btree (sha1);


--
-- Name: index_uploads_on_url; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_uploads_on_url ON public.uploads USING btree (url);


--
-- Name: index_uploads_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_uploads_on_user_id ON public.uploads USING btree (user_id);


--
-- Name: index_user_actions_on_acting_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_actions_on_acting_user_id ON public.user_actions USING btree (acting_user_id);


--
-- Name: index_user_actions_on_action_type_and_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_actions_on_action_type_and_created_at ON public.user_actions USING btree (action_type, created_at, user_id);


--
-- Name: index_user_actions_on_target_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_actions_on_target_post_id ON public.user_actions USING btree (target_post_id);


--
-- Name: index_user_actions_on_target_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_actions_on_target_user_id ON public.user_actions USING btree (target_user_id) WHERE (target_user_id IS NOT NULL);


--
-- Name: index_user_actions_on_user_id_and_action_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_actions_on_user_id_and_action_type ON public.user_actions USING btree (user_id, action_type);


--
-- Name: index_user_api_key_clients_on_client_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_user_api_key_clients_on_client_id ON public.user_api_key_clients USING btree (client_id);


--
-- Name: index_user_api_key_scopes_on_user_api_key_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_api_key_scopes_on_user_api_key_id ON public.user_api_key_scopes USING btree (user_api_key_id);


--
-- Name: index_user_api_keys_on_client_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_user_api_keys_on_client_id ON public.user_api_keys USING btree (client_id);


--
-- Name: index_user_api_keys_on_key_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_user_api_keys_on_key_hash ON public.user_api_keys USING btree (key_hash);


--
-- Name: index_user_api_keys_on_user_api_key_client_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_api_keys_on_user_api_key_client_id ON public.user_api_keys USING btree (user_api_key_client_id);


--
-- Name: index_user_api_keys_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_api_keys_on_user_id ON public.user_api_keys USING btree (user_id);


--
-- Name: index_user_archived_messages_on_user_id_and_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_user_archived_messages_on_user_id_and_topic_id ON public.user_archived_messages USING btree (user_id, topic_id);


--
-- Name: index_user_associated_groups; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_user_associated_groups ON public.user_associated_groups USING btree (user_id, associated_group_id);


--
-- Name: index_user_associated_groups_on_associated_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_associated_groups_on_associated_group_id ON public.user_associated_groups USING btree (associated_group_id);


--
-- Name: index_user_associated_groups_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_associated_groups_on_user_id ON public.user_associated_groups USING btree (user_id);


--
-- Name: index_user_auth_token_logs_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_auth_token_logs_on_user_id ON public.user_auth_token_logs USING btree (user_id);


--
-- Name: index_user_auth_tokens_on_auth_token; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_user_auth_tokens_on_auth_token ON public.user_auth_tokens USING btree (auth_token);


--
-- Name: index_user_auth_tokens_on_impersonation_expires_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_auth_tokens_on_impersonation_expires_at ON public.user_auth_tokens USING btree (impersonation_expires_at) WHERE (impersonation_expires_at IS NOT NULL);


--
-- Name: index_user_auth_tokens_on_prev_auth_token; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_user_auth_tokens_on_prev_auth_token ON public.user_auth_tokens USING btree (prev_auth_token);


--
-- Name: index_user_auth_tokens_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_auth_tokens_on_user_id ON public.user_auth_tokens USING btree (user_id);


--
-- Name: index_user_avatars_on_custom_upload_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_avatars_on_custom_upload_id ON public.user_avatars USING btree (custom_upload_id);


--
-- Name: index_user_avatars_on_gravatar_upload_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_avatars_on_gravatar_upload_id ON public.user_avatars USING btree (gravatar_upload_id);


--
-- Name: index_user_avatars_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_avatars_on_user_id ON public.user_avatars USING btree (user_id);


--
-- Name: index_user_badges_on_badge_id_and_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_badges_on_badge_id_and_user_id ON public.user_badges USING btree (badge_id, user_id);


--
-- Name: index_user_badges_on_badge_id_and_user_id_and_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_user_badges_on_badge_id_and_user_id_and_post_id ON public.user_badges USING btree (badge_id, user_id, post_id) WHERE (post_id IS NOT NULL);


--
-- Name: index_user_badges_on_badge_id_and_user_id_and_seq; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_user_badges_on_badge_id_and_user_id_and_seq ON public.user_badges USING btree (badge_id, user_id, seq) WHERE (post_id IS NULL);


--
-- Name: index_user_badges_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_badges_on_user_id ON public.user_badges USING btree (user_id);


--
-- Name: index_user_chat_channel_memberships_on_user_id_and_starred; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_chat_channel_memberships_on_user_id_and_starred ON public.user_chat_channel_memberships USING btree (user_id, starred);


--
-- Name: index_user_custom_fields_on_user_id_and_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_custom_fields_on_user_id_and_name ON public.user_custom_fields USING btree (user_id, name);


--
-- Name: index_user_custom_fields_on_value; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_user_custom_fields_on_value ON public.user_custom_fields USING btree (value) WHERE ((name)::text = 'ai-stream-conversation-unique-id'::text);


--
-- Name: index_user_emails_on_email; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_user_emails_on_email ON public.user_emails USING btree (lower((email)::text));


--
-- Name: index_user_emails_on_normalized_email; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_emails_on_normalized_email ON public.user_emails USING btree (lower((normalized_email)::text));


--
-- Name: index_user_emails_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_emails_on_user_id ON public.user_emails USING btree (user_id);


--
-- Name: index_user_emails_on_user_id_and_primary; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_user_emails_on_user_id_and_primary ON public.user_emails USING btree (user_id, "primary") WHERE "primary";


--
-- Name: index_user_histories_on_acting_user_id_and_action_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_histories_on_acting_user_id_and_action_and_id ON public.user_histories USING btree (acting_user_id, action, id);


--
-- Name: index_user_histories_on_action_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_histories_on_action_and_id ON public.user_histories USING btree (action, id);


--
-- Name: index_user_histories_on_category_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_histories_on_category_id ON public.user_histories USING btree (category_id);


--
-- Name: index_user_histories_on_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_histories_on_post_id ON public.user_histories USING btree (post_id);


--
-- Name: index_user_histories_on_reviewable_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_histories_on_reviewable_id ON public.user_histories USING btree (reviewable_id);


--
-- Name: index_user_histories_on_subject_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_histories_on_subject_and_id ON public.user_histories USING btree (subject, id);


--
-- Name: index_user_histories_on_target_user_id_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_histories_on_target_user_id_and_id ON public.user_histories USING btree (target_user_id, id);


--
-- Name: index_user_histories_on_topic_id_and_target_user_id_and_action; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_histories_on_topic_id_and_target_user_id_and_action ON public.user_histories USING btree (topic_id, target_user_id, action);


--
-- Name: index_user_ip_address_histories_on_user_id_and_ip_address; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_user_ip_address_histories_on_user_id_and_ip_address ON public.user_ip_address_histories USING btree (user_id, ip_address);


--
-- Name: index_user_notification_schedules_on_enabled; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_notification_schedules_on_enabled ON public.user_notification_schedules USING btree (enabled);


--
-- Name: index_user_notification_schedules_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_notification_schedules_on_user_id ON public.user_notification_schedules USING btree (user_id);


--
-- Name: index_user_open_ids_on_url; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_open_ids_on_url ON public.user_open_ids USING btree (url);


--
-- Name: index_user_options_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_user_options_on_user_id ON public.user_options USING btree (user_id);


--
-- Name: index_user_options_on_user_id_and_default_calendar; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_options_on_user_id_and_default_calendar ON public.user_options USING btree (user_id, default_calendar);


--
-- Name: index_user_options_on_watched_precedence_over_muted; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_options_on_watched_precedence_over_muted ON public.user_options USING btree (watched_precedence_over_muted);


--
-- Name: index_user_passwords_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_user_passwords_on_user_id ON public.user_passwords USING btree (user_id);


--
-- Name: index_user_profile_views_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_profile_views_on_user_id ON public.user_profile_views USING btree (user_id);


--
-- Name: index_user_profile_views_on_user_profile_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_profile_views_on_user_profile_id ON public.user_profile_views USING btree (user_profile_id);


--
-- Name: index_user_profiles_on_bio_cooked_version; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_profiles_on_bio_cooked_version ON public.user_profiles USING btree (bio_cooked_version);


--
-- Name: index_user_profiles_on_card_background_upload_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_profiles_on_card_background_upload_id ON public.user_profiles USING btree (card_background_upload_id);


--
-- Name: index_user_profiles_on_granted_title_badge_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_profiles_on_granted_title_badge_id ON public.user_profiles USING btree (granted_title_badge_id);


--
-- Name: index_user_profiles_on_profile_background_upload_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_profiles_on_profile_background_upload_id ON public.user_profiles USING btree (profile_background_upload_id);


--
-- Name: index_user_second_factors_on_method_and_enabled; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_second_factors_on_method_and_enabled ON public.user_second_factors USING btree (method, enabled);


--
-- Name: index_user_second_factors_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_second_factors_on_user_id ON public.user_second_factors USING btree (user_id);


--
-- Name: index_user_security_keys_on_credential_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_user_security_keys_on_credential_id ON public.user_security_keys USING btree (credential_id);


--
-- Name: index_user_security_keys_on_factor_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_security_keys_on_factor_type ON public.user_security_keys USING btree (factor_type);


--
-- Name: index_user_security_keys_on_factor_type_and_enabled; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_security_keys_on_factor_type_and_enabled ON public.user_security_keys USING btree (factor_type, enabled);


--
-- Name: index_user_security_keys_on_last_used; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_security_keys_on_last_used ON public.user_security_keys USING btree (last_used);


--
-- Name: index_user_security_keys_on_public_key; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_security_keys_on_public_key ON public.user_security_keys USING btree (public_key);


--
-- Name: index_user_security_keys_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_security_keys_on_user_id ON public.user_security_keys USING btree (user_id);


--
-- Name: index_user_uploads_on_upload_id_and_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_user_uploads_on_upload_id_and_user_id ON public.user_uploads USING btree (upload_id, user_id);


--
-- Name: index_user_uploads_on_user_id_and_upload_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_uploads_on_user_id_and_upload_id ON public.user_uploads USING btree (user_id, upload_id);


--
-- Name: index_user_visit_daily_rollups_on_date; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_user_visit_daily_rollups_on_date ON public.user_visit_daily_rollups USING btree (date);


--
-- Name: index_user_visits_on_user_id_and_visited_at; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_user_visits_on_user_id_and_visited_at ON public.user_visits USING btree (user_id, visited_at);


--
-- Name: index_user_visits_on_user_id_and_visited_at_and_time_read; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_visits_on_user_id_and_visited_at_and_time_read ON public.user_visits USING btree (user_id, visited_at, time_read);


--
-- Name: index_user_visits_on_visited_at_and_mobile; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_visits_on_visited_at_and_mobile ON public.user_visits USING btree (visited_at, mobile);


--
-- Name: index_user_warnings_on_topic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_user_warnings_on_topic_id ON public.user_warnings USING btree (topic_id);


--
-- Name: index_user_warnings_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_warnings_on_user_id ON public.user_warnings USING btree (user_id);


--
-- Name: index_users_on_last_posted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_last_posted_at ON public.users USING btree (last_posted_at);


--
-- Name: index_users_on_last_seen_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_last_seen_at ON public.users USING btree (last_seen_at);


--
-- Name: index_users_on_secure_identifier; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_users_on_secure_identifier ON public.users USING btree (secure_identifier);


--
-- Name: index_users_on_uploaded_avatar_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_uploaded_avatar_id ON public.users USING btree (uploaded_avatar_id);


--
-- Name: index_users_on_username; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_users_on_username ON public.users USING btree (username);


--
-- Name: index_users_on_username_lower; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_users_on_username_lower ON public.users USING btree (username_lower);


--
-- Name: index_voice_co_presences_on_user_id_1_and_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_voice_co_presences_on_user_id_1_and_date ON public.voice_co_presences USING btree (user_id_1, date);


--
-- Name: index_voice_co_presences_on_user_id_2_and_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_voice_co_presences_on_user_id_2_and_date ON public.voice_co_presences USING btree (user_id_2, date);


--
-- Name: index_voice_invites_on_room_id_and_user_id_and_invited_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_voice_invites_on_room_id_and_user_id_and_invited_by_id ON public.voice_invites USING btree (room_id, user_id, invited_by_id);


--
-- Name: index_voice_invites_on_user_id_and_room_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_voice_invites_on_user_id_and_room_id ON public.voice_invites USING btree (user_id, room_id);


--
-- Name: index_voice_recordings_on_egress_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_voice_recordings_on_egress_id ON public.voice_recordings USING btree (egress_id);


--
-- Name: index_voice_recordings_on_room_id_and_started_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_voice_recordings_on_room_id_and_started_at ON public.voice_recordings USING btree (room_id, started_at);


--
-- Name: index_voice_room_memberships_on_room_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_voice_room_memberships_on_room_id ON public.voice_room_memberships USING btree (room_id);


--
-- Name: index_voice_room_memberships_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_voice_room_memberships_on_user_id ON public.voice_room_memberships USING btree (user_id);


--
-- Name: index_voice_rooms_on_chat_channel_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_voice_rooms_on_chat_channel_id ON public.voice_rooms USING btree (chat_channel_id);


--
-- Name: index_voice_rooms_on_creator_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_voice_rooms_on_creator_id ON public.voice_rooms USING btree (creator_id);


--
-- Name: index_voice_rooms_on_ephemeral; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_voice_rooms_on_ephemeral ON public.voice_rooms USING btree (id) WHERE ephemeral;


--
-- Name: index_voice_rooms_on_slug; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_voice_rooms_on_slug ON public.voice_rooms USING btree (slug);


--
-- Name: index_voice_sessions_on_room_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_voice_sessions_on_room_id ON public.voice_sessions USING btree (room_id);


--
-- Name: index_voice_sessions_on_room_id_and_joined_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_voice_sessions_on_room_id_and_joined_at ON public.voice_sessions USING btree (room_id, joined_at);


--
-- Name: index_voice_sessions_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_voice_sessions_on_user_id ON public.voice_sessions USING btree (user_id);


--
-- Name: index_voice_sessions_on_user_id_and_joined_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_voice_sessions_on_user_id_and_joined_at ON public.voice_sessions USING btree (user_id, joined_at);


--
-- Name: index_voice_sessions_on_user_id_and_room_id_and_joined_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_voice_sessions_on_user_id_and_room_id_and_joined_at ON public.voice_sessions USING btree (user_id, room_id, joined_at);


--
-- Name: index_watched_words_on_action_and_word; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_watched_words_on_action_and_word ON public.watched_words USING btree (action, word);


--
-- Name: index_watched_words_on_watched_word_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_watched_words_on_watched_word_group_id ON public.watched_words USING btree (watched_word_group_id);


--
-- Name: index_web_crawler_requests_on_date_and_user_agent; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_web_crawler_requests_on_date_and_user_agent ON public.web_crawler_requests USING btree (date, user_agent);


--
-- Name: index_web_hook_events_daily_aggregates_on_web_hook_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_web_hook_events_daily_aggregates_on_web_hook_id ON public.web_hook_events_daily_aggregates USING btree (web_hook_id);


--
-- Name: index_web_hook_events_on_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_web_hook_events_on_created_at ON public.web_hook_events USING btree (created_at);


--
-- Name: index_web_hook_events_on_web_hook_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_web_hook_events_on_web_hook_id ON public.web_hook_events USING btree (web_hook_id);


--
-- Name: post_timings_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX post_timings_unique ON public.post_timings USING btree (topic_id, post_number, user_id);


--
-- Name: post_voting_comments_deleted_by_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX post_voting_comments_deleted_by_id_idx ON public.post_voting_comments USING btree (deleted_by_id) WHERE (deleted_by_id IS NOT NULL);


--
-- Name: post_voting_comments_post_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX post_voting_comments_post_id_idx ON public.post_voting_comments USING btree (post_id);


--
-- Name: post_voting_comments_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX post_voting_comments_user_id_idx ON public.post_voting_comments USING btree (user_id);


--
-- Name: post_voting_votes_votable_type_and_votable_id_and_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX post_voting_votes_votable_type_and_votable_id_and_user_id_idx ON public.post_voting_votes USING btree (votable_type, votable_id, user_id);


--
-- Name: post_voting_votes_votable_type_votable_id_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX post_voting_votes_votable_type_votable_id_user_id_idx ON public.post_voting_votes USING btree (votable_type, votable_id, user_id);


--
-- Name: reaction_id_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX reaction_id_user_id ON public.discourse_reactions_reaction_users USING btree (reaction_id, user_id);


--
-- Name: reaction_type_reaction_value; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX reaction_type_reaction_value ON public.discourse_reactions_reactions USING btree (post_id, reaction_type, reaction_value);


--
-- Name: theme_field_unique_index; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX theme_field_unique_index ON public.theme_fields USING btree (theme_id, target_id, type_id, name);


--
-- Name: theme_translation_overrides_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX theme_translation_overrides_unique ON public.theme_translation_overrides USING btree (theme_id, locale, translation_key);


--
-- Name: topic_custom_fields_value_key_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX topic_custom_fields_value_key_idx ON public.topic_custom_fields USING btree (value, name) WHERE ((value IS NOT NULL) AND (char_length(value) < 400));


--
-- Name: topic_voting_category_settings_category_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX topic_voting_category_settings_category_id_idx ON public.topic_voting_category_settings USING btree (category_id);


--
-- Name: topic_voting_topic_vote_count_topic_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX topic_voting_topic_vote_count_topic_id_idx ON public.topic_voting_topic_vote_count USING btree (topic_id);


--
-- Name: topic_voting_votes_user_id_topic_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX topic_voting_votes_user_id_topic_id_idx ON public.topic_voting_votes USING btree (user_id, topic_id);


--
-- Name: uniq_ip_or_user_id_topic_views; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uniq_ip_or_user_id_topic_views ON public.topic_views USING btree (user_id, ip_address, topic_id);


--
-- Name: unique_chat_message_links; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX unique_chat_message_links ON public.chat_message_links USING btree (chat_message_id, url);


--
-- Name: unique_classification_target_per_type; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX unique_classification_target_per_type ON public.classification_results USING btree (target_id, target_type, model_used);


--
-- Name: unique_index_categories_on_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX unique_index_categories_on_name ON public.categories USING btree (COALESCE(parent_category_id, '-1'::integer), name);


--
-- Name: unique_index_categories_on_slug; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX unique_index_categories_on_slug ON public.categories USING btree (COALESCE(parent_category_id, '-1'::integer), lower((slug)::text)) WHERE ((slug)::text <> ''::text);


--
-- Name: unique_livestream_topic_chat_channels; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX unique_livestream_topic_chat_channels ON public.livestream_topic_chat_channels USING btree (topic_id, chat_channel_id);


--
-- Name: unique_post_links; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX unique_post_links ON public.topic_links USING btree (topic_id, post_id, url);


--
-- Name: unique_profile_view_user_or_ip; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX unique_profile_view_user_or_ip ON public.user_profile_views USING btree (viewed_at, user_id, ip_address, user_profile_id);


--
-- Name: unique_target_and_assigned; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX unique_target_and_assigned ON public.assignments USING btree (assigned_to_id, assigned_to_type, target_id, target_type);


--
-- Name: unique_topic_thumbnails; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX unique_topic_thumbnails ON public.topic_thumbnails USING btree (upload_id, max_width, max_height);


--
-- Name: user_chat_channel_memberships_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_chat_channel_memberships_index ON public.user_chat_channel_memberships USING btree (user_id, chat_channel_id, notification_level, following);


--
-- Name: user_chat_channel_unique_memberships; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX user_chat_channel_unique_memberships ON public.user_chat_channel_memberships USING btree (user_id, chat_channel_id);


--
-- Name: user_chat_thread_unique_memberships; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX user_chat_thread_unique_memberships ON public.user_chat_thread_memberships USING btree (user_id, thread_id);


--
-- Name: user_id_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX user_id_post_id ON public.discourse_reactions_reaction_users USING btree (user_id, post_id);


--
-- Name: web_hooks_tags; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX web_hooks_tags ON public.tags_web_hooks USING btree (web_hook_id, tag_id);


--
-- Name: category_settings category_settings_require_reply_approval_readonly; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER category_settings_require_reply_approval_readonly BEFORE INSERT OR UPDATE OF require_reply_approval ON public.category_settings FOR EACH ROW WHEN ((new.require_reply_approval IS NOT NULL)) EXECUTE FUNCTION discourse_functions.raise_category_settings_require_reply_approval_readonly();


--
-- Name: category_settings category_settings_require_topic_approval_readonly; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER category_settings_require_topic_approval_readonly BEFORE INSERT OR UPDATE OF require_topic_approval ON public.category_settings FOR EACH ROW WHEN ((new.require_topic_approval IS NOT NULL)) EXECUTE FUNCTION discourse_functions.raise_category_settings_require_topic_approval_readonly();


--
-- Name: discourse_rss_polling_rss_feeds discourse_rss_polling_rss_feeds_author_readonly; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER discourse_rss_polling_rss_feeds_author_readonly BEFORE INSERT OR UPDATE OF author ON public.discourse_rss_polling_rss_feeds FOR EACH ROW WHEN ((new.author IS NOT NULL)) EXECUTE FUNCTION discourse_functions.raise_discourse_rss_polling_rss_feeds_author_readonly();


--
-- Name: topic_timers topic_timers_topic_id_readonly; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER topic_timers_topic_id_readonly BEFORE INSERT OR UPDATE OF topic_id ON public.topic_timers FOR EACH ROW WHEN ((new.topic_id IS NOT NULL)) EXECUTE FUNCTION discourse_functions.raise_topic_timers_topic_id_readonly();


--
-- Name: user_profiles fk_rails_1d362f2e97; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_profiles
    ADD CONSTRAINT fk_rails_1d362f2e97 FOREIGN KEY (profile_background_upload_id) REFERENCES public.uploads(id);


--
-- Name: discourse_kanban_cards fk_rails_23a074d40f; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_kanban_cards
    ADD CONSTRAINT fk_rails_23a074d40f FOREIGN KEY (column_id) REFERENCES public.discourse_kanban_columns(id) ON DELETE SET NULL;


--
-- Name: reviewable_notes fk_rails_2fe5fa5cd0; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviewable_notes
    ADD CONSTRAINT fk_rails_2fe5fa5cd0 FOREIGN KEY (reviewable_id) REFERENCES public.reviewables(id);


--
-- Name: user_profiles fk_rails_38ea484ed4; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_profiles
    ADD CONSTRAINT fk_rails_38ea484ed4 FOREIGN KEY (granted_title_badge_id) REFERENCES public.badges(id);


--
-- Name: discourse_kanban_cards fk_rails_428adf8573; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_kanban_cards
    ADD CONSTRAINT fk_rails_428adf8573 FOREIGN KEY (topic_id) REFERENCES public.topics(id) ON DELETE CASCADE;


--
-- Name: ad_plugin_impressions fk_rails_45ce2c4d3c; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ad_plugin_impressions
    ADD CONSTRAINT fk_rails_45ce2c4d3c FOREIGN KEY (ad_plugin_house_ad_id) REFERENCES public.ad_plugin_house_ads(id) ON DELETE CASCADE;


--
-- Name: ad_plugin_house_ads_groups fk_rails_4973d7060d; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ad_plugin_house_ads_groups
    ADD CONSTRAINT fk_rails_4973d7060d FOREIGN KEY (ad_plugin_house_ad_id) REFERENCES public.ad_plugin_house_ads(id) ON DELETE CASCADE;


--
-- Name: discourse_kanban_cards fk_rails_51c00a5eb2; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_kanban_cards
    ADD CONSTRAINT fk_rails_51c00a5eb2 FOREIGN KEY (board_id) REFERENCES public.discourse_kanban_boards(id) ON DELETE CASCADE;


--
-- Name: javascript_caches fk_rails_58f94aecc4; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.javascript_caches
    ADD CONSTRAINT fk_rails_58f94aecc4 FOREIGN KEY (theme_id) REFERENCES public.themes(id) ON DELETE CASCADE;


--
-- Name: optimized_videos fk_rails_7c76beeaf4; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.optimized_videos
    ADD CONSTRAINT fk_rails_7c76beeaf4 FOREIGN KEY (optimized_upload_id) REFERENCES public.uploads(id);


--
-- Name: poll_votes fk_rails_848ece0184; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.poll_votes
    ADD CONSTRAINT fk_rails_848ece0184 FOREIGN KEY (poll_option_id) REFERENCES public.poll_options(id);


--
-- Name: optimized_videos fk_rails_84f2496311; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.optimized_videos
    ADD CONSTRAINT fk_rails_84f2496311 FOREIGN KEY (upload_id) REFERENCES public.uploads(id);


--
-- Name: user_security_keys fk_rails_90999b0454; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_security_keys
    ADD CONSTRAINT fk_rails_90999b0454 FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: reviewable_notes fk_rails_9ea278a8aa; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviewable_notes
    ADD CONSTRAINT fk_rails_9ea278a8aa FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: voice_room_memberships fk_rails_a55e8c404b; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.voice_room_memberships
    ADD CONSTRAINT fk_rails_a55e8c404b FOREIGN KEY (room_id) REFERENCES public.voice_rooms(id);


--
-- Name: poll_votes fk_rails_a6e6974b7e; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.poll_votes
    ADD CONSTRAINT fk_rails_a6e6974b7e FOREIGN KEY (poll_id) REFERENCES public.polls(id);


--
-- Name: poll_options fk_rails_aa85becb42; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.poll_options
    ADD CONSTRAINT fk_rails_aa85becb42 FOREIGN KEY (poll_id) REFERENCES public.polls(id);


--
-- Name: ad_plugin_house_ads_routes fk_rails_b126a33930; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ad_plugin_house_ads_routes
    ADD CONSTRAINT fk_rails_b126a33930 FOREIGN KEY (ad_plugin_house_ad_id) REFERENCES public.ad_plugin_house_ads(id);


--
-- Name: polls fk_rails_b50b782d08; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.polls
    ADD CONSTRAINT fk_rails_b50b782d08 FOREIGN KEY (post_id) REFERENCES public.posts(id);


--
-- Name: poll_votes fk_rails_b64de9b025; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.poll_votes
    ADD CONSTRAINT fk_rails_b64de9b025 FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: ai_tool_actions fk_rails_bf8a76772d; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_tool_actions
    ADD CONSTRAINT fk_rails_bf8a76772d FOREIGN KEY (ai_agent_id) REFERENCES public.ai_agents(id);


--
-- Name: discourse_kanban_columns fk_rails_c2f2ed5c5d; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_kanban_columns
    ADD CONSTRAINT fk_rails_c2f2ed5c5d FOREIGN KEY (move_to_category_id) REFERENCES public.categories(id) ON DELETE SET NULL;


--
-- Name: ad_plugin_house_ads_categories fk_rails_c6e88d8af5; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ad_plugin_house_ads_categories
    ADD CONSTRAINT fk_rails_c6e88d8af5 FOREIGN KEY (category_id) REFERENCES public.categories(id) ON DELETE CASCADE;


--
-- Name: user_profiles fk_rails_ca64aa462b; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_profiles
    ADD CONSTRAINT fk_rails_ca64aa462b FOREIGN KEY (card_background_upload_id) REFERENCES public.uploads(id);


--
-- Name: ad_plugin_house_ads_categories fk_rails_ea323de4ce; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ad_plugin_house_ads_categories
    ADD CONSTRAINT fk_rails_ea323de4ce FOREIGN KEY (ad_plugin_house_ad_id) REFERENCES public.ad_plugin_house_ads(id) ON DELETE CASCADE;


--
-- Name: javascript_caches fk_rails_ed33506dbd; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.javascript_caches
    ADD CONSTRAINT fk_rails_ed33506dbd FOREIGN KEY (theme_field_id) REFERENCES public.theme_fields(id) ON DELETE CASCADE;


--
-- Name: discourse_kanban_columns fk_rails_ef6a319cc1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.discourse_kanban_columns
    ADD CONSTRAINT fk_rails_ef6a319cc1 FOREIGN KEY (board_id) REFERENCES public.discourse_kanban_boards(id) ON DELETE CASCADE;


--
-- Name: ad_plugin_impressions fk_rails_f446846ed4; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ad_plugin_impressions
    ADD CONSTRAINT fk_rails_f446846ed4 FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: ad_plugin_house_ads_groups fk_rails_fcbec7868d; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ad_plugin_house_ads_groups
    ADD CONSTRAINT fk_rails_fcbec7868d FOREIGN KEY (group_id) REFERENCES public.groups(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--


