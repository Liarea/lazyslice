--
-- PostgreSQL database dump
--


-- Dumped from database version 16.15 (Debian 16.15-1.pgdg13+2)
-- Dumped by pg_dump version 16.15 (Debian 16.15-1.pgdg13+2)

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
-- Name: gitlab_partitions_dynamic; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA gitlab_partitions_dynamic;


--
-- Name: SCHEMA gitlab_partitions_dynamic; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON SCHEMA gitlab_partitions_dynamic IS 'Schema to hold partitions managed dynamically from the application, e.g. for time space partitioning.';


--
-- Name: gitlab_partitions_static; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA gitlab_partitions_static;


--
-- Name: SCHEMA gitlab_partitions_static; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON SCHEMA gitlab_partitions_static IS 'Schema to hold static partitions, e.g. for hash partitioning';


--
-- Name: btree_gist; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS btree_gist WITH SCHEMA public;


--
-- Name: EXTENSION btree_gist; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION btree_gist IS 'support for indexing common datatypes in GiST';


--
-- Name: pg_trgm; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;


--
-- Name: EXTENSION pg_trgm; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION pg_trgm IS 'text similarity measurement and index searching based on trigrams';


--
-- Name: assign_ci_runner_controller_runner_level_scopings_id_value(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.assign_ci_runner_controller_runner_level_scopings_id_value() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."id" IS NOT NULL THEN
  RAISE WARNING 'Manually assigning ids is not allowed, the value will be ignored';
END IF;
NEW."id" := nextval('ci_runner_controller_runner_level_scopings_id_seq'::regclass);
RETURN NEW;

END
$$;


--
-- Name: assign_ci_runner_machines_id_value(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.assign_ci_runner_machines_id_value() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."id" IS NOT NULL THEN
  RAISE WARNING 'Manually assigning ids is not allowed, the value will be ignored';
END IF;
NEW."id" := nextval('ci_runner_machines_id_seq'::regclass);
RETURN NEW;

END
$$;


--
-- Name: assign_ci_runner_taggings_id_value(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.assign_ci_runner_taggings_id_value() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."id" IS NOT NULL THEN
  RAISE WARNING 'Manually assigning ids is not allowed, the value will be ignored';
END IF;
NEW."id" := nextval('ci_runner_taggings_id_seq'::regclass);
RETURN NEW;

END
$$;


--
-- Name: assign_ci_runners_id_value(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.assign_ci_runners_id_value() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."id" IS NOT NULL THEN
  RAISE WARNING 'Manually assigning ids is not allowed, the value will be ignored';
END IF;
NEW."id" := nextval('ci_runners_id_seq'::regclass);
RETURN NEW;

END
$$;


--
-- Name: assign_p_ci_builds_id_value(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.assign_p_ci_builds_id_value() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."id" IS NOT NULL THEN
  RAISE WARNING 'Manually assigning ids is not allowed, the value will be ignored';
END IF;
NEW."id" := nextval('ci_builds_id_seq'::regclass);
RETURN NEW;

END
$$;


--
-- Name: assign_p_ci_job_annotations_id_value(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.assign_p_ci_job_annotations_id_value() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."id" IS NOT NULL THEN
  RAISE WARNING 'Manually assigning ids is not allowed, the value will be ignored';
END IF;
NEW."id" := nextval('p_ci_job_annotations_id_seq'::regclass);
RETURN NEW;

END
$$;


--
-- Name: assign_p_ci_job_artifacts_id_value(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.assign_p_ci_job_artifacts_id_value() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."id" IS NOT NULL THEN
  RAISE WARNING 'Manually assigning ids is not allowed, the value will be ignored';
END IF;
NEW."id" := nextval('ci_job_artifacts_id_seq'::regclass);
RETURN NEW;

END
$$;


--
-- Name: assign_p_ci_pipeline_variables_id_value(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.assign_p_ci_pipeline_variables_id_value() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."id" IS NOT NULL THEN
  RAISE WARNING 'Manually assigning ids is not allowed, the value will be ignored';
END IF;
NEW."id" := nextval('ci_pipeline_variables_id_seq'::regclass);
RETURN NEW;

END
$$;


--
-- Name: assign_p_ci_pipelines_id_value(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.assign_p_ci_pipelines_id_value() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."id" IS NOT NULL THEN
  RAISE WARNING 'Manually assigning ids is not allowed, the value will be ignored';
END IF;
NEW."id" := nextval('ci_pipelines_id_seq'::regclass);
RETURN NEW;

END
$$;


--
-- Name: assign_p_ci_stages_id_value(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.assign_p_ci_stages_id_value() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."id" IS NOT NULL THEN
  RAISE WARNING 'Manually assigning ids is not allowed, the value will be ignored';
END IF;
NEW."id" := nextval('ci_stages_id_seq'::regclass);
RETURN NEW;

END
$$;


--
-- Name: assign_p_duo_workflows_checkpoint_blobs_id_value(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.assign_p_duo_workflows_checkpoint_blobs_id_value() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."id" IS NOT NULL THEN
  RAISE WARNING 'Manually assigning ids is not allowed, the value will be ignored';
END IF;
NEW."id" := nextval('p_duo_workflows_checkpoint_blobs_id_seq'::regclass);
RETURN NEW;

END
$$;


--
-- Name: assign_p_duo_workflows_checkpoint_headers_id_value(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.assign_p_duo_workflows_checkpoint_headers_id_value() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."id" IS NOT NULL THEN
  RAISE WARNING 'Manually assigning ids is not allowed, the value will be ignored';
END IF;
NEW."id" := nextval('p_duo_workflows_checkpoint_headers_id_seq'::regclass);
RETURN NEW;

END
$$;


--
-- Name: assign_p_duo_workflows_checkpoints_id_value(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.assign_p_duo_workflows_checkpoints_id_value() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."id" IS NOT NULL THEN
  RAISE WARNING 'Manually assigning ids is not allowed, the value will be ignored';
END IF;
NEW."id" := nextval('p_duo_workflows_checkpoints_id_seq'::regclass);
RETURN NEW;

END
$$;


--
-- Name: assign_p_knowledge_graph_code_indexing_tasks_id_value(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.assign_p_knowledge_graph_code_indexing_tasks_id_value() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."id" IS NOT NULL THEN
  RAISE WARNING 'Manually assigning ids is not allowed, the value will be ignored';
END IF;
NEW."id" := nextval('p_knowledge_graph_code_indexing_tasks_id_seq'::regclass);
RETURN NEW;

END
$$;


--
-- Name: assign_zoekt_tasks_id_value(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.assign_zoekt_tasks_id_value() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."id" IS NOT NULL THEN
  RAISE WARNING 'Manually assigning ids is not allowed, the value will be ignored';
END IF;
NEW."id" := nextval('zoekt_tasks_id_seq'::regclass);
RETURN NEW;

END
$$;


--
-- Name: bulk_import_batch_trackers_sharding_key(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.bulk_import_batch_trackers_sharding_key() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  IF num_nonnulls(NEW.organization_id, NEW.namespace_id, NEW.project_id) != 1 THEN
    SELECT "organization_id", "namespace_id", "project_id"
    INTO NEW."organization_id", NEW."namespace_id", NEW."project_id"
    FROM "bulk_import_trackers"
    WHERE "bulk_import_trackers"."id" = NEW."tracker_id";
  END IF;

  RETURN NEW;
END
$$;


--
-- Name: bulk_import_trackers_sharding_key(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.bulk_import_trackers_sharding_key() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  IF num_nonnulls(NEW.namespace_id, NEW.organization_id, NEW.project_id) != 1 THEN
    SELECT "organization_id", "namespace_id", "project_id"
    INTO NEW."organization_id", NEW."namespace_id", NEW."project_id"
    FROM "bulk_import_entities"
    WHERE "bulk_import_entities"."id" = NEW."bulk_import_entity_id";
  END IF;

  RETURN NEW;
END
$$;


--
-- Name: check_work_item_custom_type_exists(bigint); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.check_work_item_custom_type_exists(custom_type_id bigint) RETURNS boolean
    LANGUAGE plpgsql COST 1 PARALLEL SAFE
    AS $_$
BEGIN
  PERFORM 1
  FROM work_item_custom_types
  WHERE id = $1
  FOR KEY SHARE;

  RETURN FOUND;
END;
$_$;


--
-- Name: cleanup_pipeline_iid_after_delete(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.cleanup_pipeline_iid_after_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  IF OLD.iid IS NOT NULL THEN
    DELETE FROM p_ci_pipeline_iids
    WHERE project_id = OLD.project_id AND iid = OLD.iid;
  END IF;
  RETURN OLD;
END;
$$;


--
-- Name: cluster_platforms_kubernetes_sharding_key(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.cluster_platforms_kubernetes_sharding_key() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  IF num_nonnulls(NEW.organization_id, NEW.group_id, NEW.project_id) != 1 THEN
    SELECT "organization_id", "group_id", "project_id"
    INTO NEW."organization_id", NEW."group_id", NEW."project_id"
    FROM "clusters"
    WHERE "clusters"."id" = NEW."cluster_id";
  END IF;

  RETURN NEW;
END
$$;


--
-- Name: cluster_providers_aws_sharding_key(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.cluster_providers_aws_sharding_key() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  IF num_nonnulls(NEW.organization_id, NEW.group_id, NEW.project_id) != 1 THEN
    SELECT "organization_id", "group_id", "project_id"
    INTO NEW."organization_id", NEW."group_id", NEW."project_id"
    FROM "clusters"
    WHERE "clusters"."id" = NEW."cluster_id";
  END IF;

  RETURN NEW;
END
$$;


--
-- Name: cluster_providers_gcp_sharding_key(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.cluster_providers_gcp_sharding_key() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  IF num_nonnulls(NEW.organization_id, NEW.group_id, NEW.project_id) != 1 THEN
    SELECT "organization_id", "group_id", "project_id"
    INTO NEW."organization_id", NEW."group_id", NEW."project_id"
    FROM "clusters"
    WHERE "clusters"."id" = NEW."cluster_id";
  END IF;

  RETURN NEW;
END
$$;


--
-- Name: clusters_kubernetes_namespaces_sharding_key(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.clusters_kubernetes_namespaces_sharding_key() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  IF num_nonnulls(NEW.organization_id, NEW.group_id, NEW.sharding_project_id) != 1 THEN
    SELECT "organization_id", "group_id", "project_id"
    INTO NEW."organization_id", NEW."group_id", NEW."sharding_project_id"
    FROM "clusters"
    WHERE "clusters"."id" = NEW."cluster_id";
  END IF;

  RETURN NEW;
END
$$;


--
-- Name: custom_dashboard_search_vector_update(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.custom_dashboard_search_vector_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  INSERT INTO custom_dashboard_search_data (
    custom_dashboard_id,
    organization_id,
    name,
    description,
    search_vector,
    created_at,
    updated_at
  )
  VALUES (
    NEW.id,
    NEW.organization_id,
    coalesce(NEW.name, ''),
    coalesce(NEW.description, ''),
    to_tsvector('english', coalesce(NEW.name, '') || ' ' || coalesce(NEW.description, '')),
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
  )
  ON CONFLICT (custom_dashboard_id) DO UPDATE
  SET name          = EXCLUDED.name,
      description   = EXCLUDED.description,
      search_vector = EXCLUDED.search_vector,
      updated_at    = CURRENT_TIMESTAMP;

  RETURN NEW;
END
$$;


--
-- Name: delete_associated_project_namespace(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.delete_associated_project_namespace() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
DELETE FROM namespaces
WHERE namespaces.id = OLD.project_namespace_id AND
namespaces.type = 'Project';
RETURN NULL;

END
$$;


--
-- Name: delete_orphaned_granular_scopes(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.delete_orphaned_granular_scopes() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
DELETE FROM granular_scopes
WHERE id = OLD.granular_scope_id
AND NOT EXISTS (
  SELECT 1
  FROM personal_access_token_granular_scopes
  WHERE granular_scope_id = OLD.granular_scope_id
);
RETURN OLD;

END
$$;


--
-- Name: enqueue_gsm_deprovision_task(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.enqueue_gsm_deprovision_task() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF OLD.group_id IS NULL THEN
  RETURN NULL;
END IF;

INSERT INTO group_secrets_manager_maintenance_tasks (
  action,
  retry_count,
  organization_id,
  group_id,
  root_namespace_id
) VALUES (
  1,
  0,
  OLD.organization_id,
  OLD.group_id,
  OLD.root_namespace_id
)
ON CONFLICT (group_id) DO NOTHING;

RETURN NULL;

END
$$;


--
-- Name: enqueue_psm_deprovision_task(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.enqueue_psm_deprovision_task() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF OLD.project_id IS NULL THEN
  RETURN NULL;
END IF;

INSERT INTO project_secrets_manager_maintenance_tasks (
  action,
  retry_count,
  organization_id,
  project_id,
  root_namespace_id
) VALUES (
  1,
  0,
  OLD.organization_id,
  OLD.project_id,
  OLD.root_namespace_id
)
ON CONFLICT (project_id) DO NOTHING;

RETURN NULL;

END
$$;


--
-- Name: ensure_note_diff_files_sharding_key(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.ensure_note_diff_files_sharding_key() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
  note_project_id BIGINT;
  note_namespace_id BIGINT;
BEGIN
  SELECT "project_id", "namespace_id"
  INTO note_project_id, note_namespace_id
  FROM "notes"
  WHERE "id" = NEW."diff_note_id";

  IF note_project_id IS NOT NULL THEN
    SELECT "project_namespace_id" FROM "projects"
    INTO NEW."namespace_id" WHERE "projects"."id" = note_project_id;
  ELSE
    NEW."namespace_id" := note_namespace_id;
  END IF;

  RETURN NEW;
END
$$;


--
-- Name: ensure_pipeline_iid_uniqueness_before_insert(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.ensure_pipeline_iid_uniqueness_before_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  IF NEW.iid IS NOT NULL THEN
    BEGIN
      INSERT INTO p_ci_pipeline_iids (project_id, iid)
      VALUES (NEW.project_id, NEW.iid);
    EXCEPTION WHEN unique_violation THEN
      RAISE EXCEPTION 'Pipeline with iid % already exists for project %',
        NEW.iid, NEW.project_id
        USING ERRCODE = 'unique_violation',
              DETAIL = 'The iid must be unique within a project',
              HINT = 'Use a different iid or let the system generate one';
    END;
  END IF;

  RETURN NEW;
END;
$$;


--
-- Name: ensure_pipeline_iid_uniqueness_before_update_iid(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.ensure_pipeline_iid_uniqueness_before_update_iid() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  IF NEW.iid IS DISTINCT FROM OLD.iid THEN
    IF NEW.iid IS NOT NULL THEN
      BEGIN
        INSERT INTO p_ci_pipeline_iids (project_id, iid)
        VALUES (NEW.project_id, NEW.iid);
      EXCEPTION WHEN unique_violation THEN
        RAISE EXCEPTION 'Pipeline with iid % already exists for project %',
          NEW.iid, NEW.project_id
          USING ERRCODE = 'unique_violation',
                DETAIL = 'The iid must be unique within a project',
                HINT = 'Use a different iid or let the system generate one';
      END;
    END IF;

    IF OLD.iid IS NOT NULL THEN
      DELETE FROM p_ci_pipeline_iids
      WHERE project_id = OLD.project_id AND iid = OLD.iid;
    END IF;
  END IF;
  RETURN NEW;
END;
$$;


--
-- Name: exists_issues_for_work_item_custom_type(bigint); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.exists_issues_for_work_item_custom_type(work_item_type_id bigint) RETURNS boolean
    LANGUAGE plpgsql STABLE COST 1 PARALLEL SAFE
    AS $_$
BEGIN
  PERFORM 1
  FROM "issues"
  WHERE "issues"."work_item_type_id" = $1
  LIMIT 1;

  RETURN FOUND;
END;
$_$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: namespaces; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.namespaces (
    id bigint NOT NULL,
    name character varying NOT NULL,
    path character varying NOT NULL,
    owner_id bigint,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    type character varying DEFAULT 'User'::character varying NOT NULL,
    avatar character varying,
    membership_lock boolean DEFAULT false,
    share_with_group_lock boolean DEFAULT false,
    visibility_level integer DEFAULT 20 NOT NULL,
    request_access_enabled boolean DEFAULT true NOT NULL,
    ldap_sync_status character varying DEFAULT 'ready'::character varying NOT NULL,
    ldap_sync_error character varying,
    ldap_sync_last_update_at timestamp without time zone,
    ldap_sync_last_successful_update_at timestamp without time zone,
    ldap_sync_last_sync_at timestamp without time zone,
    lfs_enabled boolean,
    parent_id bigint,
    shared_runners_minutes_limit integer,
    repository_size_limit bigint,
    require_two_factor_authentication boolean DEFAULT false NOT NULL,
    two_factor_grace_period integer DEFAULT 48 NOT NULL,
    project_creation_level integer,
    runners_token character varying,
    file_template_project_id bigint,
    saml_discovery_token character varying,
    runners_token_encrypted character varying,
    custom_project_templates_group_id bigint,
    auto_devops_enabled boolean,
    extra_shared_runners_minutes_limit integer,
    last_ci_minutes_notification_at timestamp with time zone,
    last_ci_minutes_usage_notification_level integer,
    subgroup_creation_level integer DEFAULT 1,
    max_pages_size integer,
    max_artifacts_size integer,
    mentions_disabled boolean,
    default_branch_protection smallint,
    max_personal_access_token_lifetime integer,
    shared_runners_enabled boolean DEFAULT true NOT NULL,
    allow_descendants_override_disabled_shared_runners boolean DEFAULT false NOT NULL,
    traversal_ids bigint[] DEFAULT '{}'::bigint[] NOT NULL,
    organization_id bigint,
    state smallint DEFAULT 0,
    CONSTRAINT check_2eae3bdf93 CHECK ((organization_id IS NOT NULL)),
    CONSTRAINT check_9d490f2140 CHECK ((state IS NOT NULL))
);


--
-- Name: find_namespaces_by_id(bigint); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.find_namespaces_by_id(namespaces_id bigint) RETURNS public.namespaces
    LANGUAGE plpgsql STABLE COST 1 PARALLEL SAFE
    AS $$
BEGIN
  return (SELECT namespaces FROM namespaces WHERE id = namespaces_id LIMIT 1);
END;
$$;


--
-- Name: find_namespaces_by_id_and_organization_id(bigint, bigint); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.find_namespaces_by_id_and_organization_id(namespaces_id bigint, sharding_organization_id bigint) RETURNS public.namespaces
    LANGUAGE plpgsql STABLE COST 1 PARALLEL SAFE
    AS $$
BEGIN
  return (SELECT namespaces FROM namespaces WHERE id = namespaces_id AND organization_id = sharding_organization_id LIMIT 1);
END;
$$;


--
-- Name: projects; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.projects (
    id bigint NOT NULL,
    name character varying,
    path character varying,
    description text,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    creator_id bigint,
    namespace_id bigint NOT NULL,
    last_activity_at timestamp without time zone,
    import_url character varying,
    visibility_level integer DEFAULT 0 NOT NULL,
    archived boolean DEFAULT false NOT NULL,
    avatar character varying,
    merge_requests_template text,
    star_count integer DEFAULT 0 NOT NULL,
    merge_requests_rebase_enabled boolean DEFAULT false,
    import_type character varying,
    import_source character varying,
    approvals_before_merge integer DEFAULT 0 NOT NULL,
    reset_approvals_on_push boolean DEFAULT true,
    merge_requests_ff_only_enabled boolean DEFAULT false,
    issues_template text,
    mirror boolean DEFAULT false NOT NULL,
    mirror_last_update_at timestamp without time zone,
    mirror_last_successful_update_at timestamp without time zone,
    mirror_user_id bigint,
    shared_runners_enabled boolean DEFAULT true NOT NULL,
    runners_token character varying,
    build_allow_git_fetch boolean DEFAULT true NOT NULL,
    build_timeout integer DEFAULT 3600 NOT NULL,
    mirror_trigger_builds boolean DEFAULT false NOT NULL,
    pending_delete boolean DEFAULT false,
    public_builds boolean DEFAULT true NOT NULL,
    last_repository_check_failed boolean,
    last_repository_check_at timestamp without time zone,
    only_allow_merge_if_pipeline_succeeds boolean DEFAULT false NOT NULL,
    has_external_issue_tracker boolean,
    repository_storage character varying DEFAULT 'default'::character varying NOT NULL,
    repository_read_only boolean,
    request_access_enabled boolean DEFAULT true NOT NULL,
    has_external_wiki boolean,
    ci_config_path character varying,
    lfs_enabled boolean,
    description_html text,
    only_allow_merge_if_all_discussions_are_resolved boolean,
    repository_size_limit bigint,
    printing_merge_request_link_enabled boolean DEFAULT true NOT NULL,
    auto_cancel_pending_pipelines integer DEFAULT 1 NOT NULL,
    service_desk_enabled boolean DEFAULT true,
    cached_markdown_version integer,
    last_repository_updated_at timestamp without time zone,
    disable_overriding_approvers_per_merge_request boolean,
    storage_version smallint,
    resolve_outdated_diff_discussions boolean,
    remote_mirror_available_overridden boolean,
    only_mirror_protected_branches boolean,
    pull_mirror_available_overridden boolean,
    jobs_cache_index integer,
    external_authorization_classification_label character varying,
    mirror_overwrites_diverged_branches boolean,
    pages_https_only boolean DEFAULT true,
    external_webhook_token character varying,
    packages_enabled boolean,
    merge_requests_author_approval boolean DEFAULT false,
    pool_repository_id bigint,
    runners_token_encrypted character varying,
    bfg_object_map character varying,
    detected_repository_languages boolean,
    merge_requests_disable_committers_approval boolean,
    require_password_to_approve boolean,
    emails_disabled boolean,
    max_pages_size integer,
    max_artifacts_size integer,
    pull_mirror_branch_prefix character varying(50),
    remove_source_branch_after_merge boolean,
    marked_for_deletion_at date,
    marked_for_deletion_by_user_id bigint,
    autoclose_referenced_issues boolean,
    suggestion_commit_message character varying(255),
    project_namespace_id bigint,
    hidden boolean DEFAULT false NOT NULL,
    organization_id bigint,
    CONSTRAINT check_1a6f946a8a CHECK ((organization_id IS NOT NULL)),
    CONSTRAINT check_fa75869cb1 CHECK ((project_namespace_id IS NOT NULL))
);


--
-- Name: find_projects_by_id(bigint); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.find_projects_by_id(projects_id bigint) RETURNS public.projects
    LANGUAGE plpgsql STABLE COST 1 PARALLEL SAFE
    AS $$
BEGIN
  return (SELECT projects FROM projects WHERE id = projects_id LIMIT 1);
END;
$$;


--
-- Name: find_projects_by_id_and_organization_id(bigint, bigint); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.find_projects_by_id_and_organization_id(projects_id bigint, sharding_organization_id bigint) RETURNS public.projects
    LANGUAGE plpgsql STABLE COST 1 PARALLEL SAFE
    AS $$
BEGIN
  return (SELECT projects FROM projects WHERE id = projects_id AND organization_id = sharding_organization_id LIMIT 1);
END;
$$;


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id bigint NOT NULL,
    email character varying DEFAULT ''::character varying NOT NULL,
    encrypted_password character varying DEFAULT ''::character varying NOT NULL,
    reset_password_token character varying,
    reset_password_sent_at timestamp without time zone,
    remember_created_at timestamp without time zone,
    sign_in_count integer DEFAULT 0,
    current_sign_in_at timestamp without time zone,
    last_sign_in_at timestamp without time zone,
    current_sign_in_ip character varying,
    last_sign_in_ip character varying,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    name character varying,
    admin boolean DEFAULT false NOT NULL,
    projects_limit integer NOT NULL,
    failed_attempts integer DEFAULT 0,
    locked_at timestamp without time zone,
    username character varying,
    can_create_group boolean DEFAULT true NOT NULL,
    can_create_team boolean DEFAULT true NOT NULL,
    state character varying,
    color_scheme_id bigint DEFAULT 1 NOT NULL,
    password_expires_at timestamp without time zone,
    created_by_id bigint,
    last_credential_check_at timestamp without time zone,
    avatar character varying,
    confirmation_token character varying,
    confirmed_at timestamp without time zone,
    confirmation_sent_at timestamp without time zone,
    unconfirmed_email character varying,
    hide_no_ssh_key boolean DEFAULT false,
    admin_email_unsubscribed_at timestamp without time zone,
    notification_email character varying,
    hide_no_password boolean DEFAULT false,
    password_automatically_set boolean DEFAULT false,
    encrypted_otp_secret character varying,
    encrypted_otp_secret_iv character varying,
    encrypted_otp_secret_salt character varying,
    otp_required_for_login boolean DEFAULT false NOT NULL,
    otp_backup_codes text,
    public_email character varying,
    dashboard integer DEFAULT 0,
    project_view integer DEFAULT 2,
    consumed_timestep integer,
    layout integer DEFAULT 0,
    hide_project_limit boolean DEFAULT false,
    note text,
    unlock_token character varying,
    otp_grace_period_started_at timestamp without time zone,
    external boolean DEFAULT false,
    incoming_email_token character varying,
    auditor boolean DEFAULT false NOT NULL,
    require_two_factor_authentication_from_group boolean DEFAULT false NOT NULL,
    two_factor_grace_period integer DEFAULT 48 NOT NULL,
    last_activity_on date,
    notified_of_own_activity boolean DEFAULT false,
    preferred_language character varying,
    theme_id smallint,
    accepted_term_id bigint,
    feed_token character varying,
    private_profile boolean DEFAULT false NOT NULL,
    roadmap_layout smallint,
    include_private_contributions boolean,
    commit_email character varying,
    group_view integer,
    managing_group_id bigint,
    first_name character varying(255),
    last_name character varying(255),
    static_object_token character varying(255),
    user_type smallint DEFAULT 0,
    static_object_token_encrypted text,
    otp_secret_expires_at timestamp with time zone,
    onboarding_in_progress boolean DEFAULT false NOT NULL,
    color_mode_id smallint DEFAULT 1 NOT NULL,
    composite_identity_enforced boolean DEFAULT false NOT NULL,
    organization_id bigint NOT NULL,
    otp_secret text,
    CONSTRAINT check_061f6f1c91 CHECK ((project_view IS NOT NULL)),
    CONSTRAINT check_0dd5948e38 CHECK ((user_type IS NOT NULL)),
    CONSTRAINT check_3a60c18afc CHECK ((hide_no_password IS NOT NULL)),
    CONSTRAINT check_693c6f3aab CHECK ((hide_no_ssh_key IS NOT NULL)),
    CONSTRAINT check_7bde697e8e CHECK ((char_length(static_object_token_encrypted) <= 255)),
    CONSTRAINT check_c737c04b87 CHECK ((notified_of_own_activity IS NOT NULL)),
    CONSTRAINT check_d0b84b7b3a CHECK ((char_length(otp_secret) <= 255))
);


--
-- Name: find_users_by_id(bigint); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.find_users_by_id(users_id bigint) RETURNS public.users
    LANGUAGE plpgsql STABLE COST 1 PARALLEL SAFE
    AS $$
BEGIN
  return (SELECT users FROM users WHERE id = users_id LIMIT 1);
END;
$$;


--
-- Name: find_users_by_id_and_organization_id(bigint, bigint); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.find_users_by_id_and_organization_id(users_id bigint, sharding_organization_id bigint) RETURNS public.users
    LANGUAGE plpgsql STABLE COST 1 PARALLEL SAFE
    AS $$
BEGIN
  return (SELECT users FROM users WHERE id = users_id AND organization_id = sharding_organization_id LIMIT 1);
END;
$$;


--
-- Name: function_for_trigger_03be0f8add7e(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.function_for_trigger_03be0f8add7e() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  NEW."all_unarchived_project_ids" := NEW."all_active_project_ids";
  RETURN NEW;
END
$$;


--
-- Name: function_for_trigger_7d6a4f5b82c2(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.function_for_trigger_7d6a4f5b82c2() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  NEW."all_active_project_ids" := NEW."all_unarchived_project_ids";
  RETURN NEW;
END
$$;


--
-- Name: function_for_trigger_de99bb993511(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.function_for_trigger_de99bb993511() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  IF NEW."all_active_project_ids" IS NOT DISTINCT FROM '{}' AND NEW."all_unarchived_project_ids" IS DISTINCT FROM '{}' THEN
    NEW."all_active_project_ids" = NEW."all_unarchived_project_ids";
  END IF;

  IF NEW."all_unarchived_project_ids" IS NOT DISTINCT FROM '{}' AND NEW."all_active_project_ids" IS DISTINCT FROM '{}' THEN
    NEW."all_unarchived_project_ids" = NEW."all_active_project_ids";
  END IF;

  RETURN NEW;
END
$$;


--
-- Name: gen_random_uuid_v7(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.gen_random_uuid_v7() RETURNS uuid
    LANGUAGE plpgsql PARALLEL SAFE
    AS $$
DECLARE
  ts_ms bigint;
  sub_ms int;
  unix_ts_ms bytea;
  uuid_bytes bytea;
  now_epoch double precision;
BEGIN
  now_epoch := extract(epoch from clock_timestamp()) * 1000;
  ts_ms := floor(now_epoch)::bigint;
  sub_ms := floor((now_epoch - ts_ms) * 4096)::int;

  unix_ts_ms := substring(int8send(ts_ms) from 3);
  uuid_bytes := uuid_send(gen_random_uuid());
  uuid_bytes := overlay(uuid_bytes placing unix_ts_ms from 1 for 6);

  uuid_bytes := set_byte(uuid_bytes, 6, ((sub_ms >> 8) & x'0F'::int) | x'70'::int);
  uuid_bytes := set_byte(uuid_bytes, 7, sub_ms & x'FF'::int);

  RETURN encode(uuid_bytes, 'hex')::uuid;
END
$$;


--
-- Name: gitlab_schema_prevent_write(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.gitlab_schema_prevent_write() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF COALESCE(NULLIF(current_setting(CONCAT('lock_writes.', TG_TABLE_NAME), true), ''), 'true') THEN
      RAISE EXCEPTION 'Table: "%" is write protected within this Gitlab database.', TG_TABLE_NAME
        USING ERRCODE = 'modifying_sql_data_not_permitted',
        HINT = 'Make sure you are using the right database connection';
    END IF;
    RETURN NEW;
END
$$;


--
-- Name: heal_ci_runner_taggings_tag_id(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.heal_ci_runner_taggings_tag_id() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW.tag_id IS NULL AND NEW.tag_name IS NOT NULL THEN
  INSERT INTO tags (name)
  VALUES (NEW.tag_name)
  ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
  RETURNING id INTO NEW.tag_id;
END IF;

RETURN NEW;

END
$$;


--
-- Name: insert_catalog_resource_sync_event(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.insert_catalog_resource_sync_event() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
INSERT INTO p_catalog_resource_sync_events (catalog_resource_id, project_id)
SELECT id, OLD.id FROM catalog_resources
WHERE project_id = OLD.id;
RETURN NULL;

END
$$;


--
-- Name: insert_into_loose_foreign_keys_deleted_records(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.insert_into_loose_foreign_keys_deleted_records() RETURNS trigger
    LANGUAGE plpgsql
    AS $_$
DECLARE
  targets JSONB := CASE WHEN TG_NARGS > 0 THEN TG_ARGV[0]::jsonb ELSE '[]'::jsonb END;
  tracked_table_identifier TEXT := TG_TABLE_SCHEMA || '.' || TG_TABLE_NAME;
  target_table      TEXT;
  target_column     TEXT;
  source_column     TEXT;
  cell_local_filter TEXT;
BEGIN
  -- Cell local: no routing targets configured
  IF jsonb_array_length(targets) = 0 THEN
    INSERT INTO loose_foreign_keys_deleted_records
    (fully_qualified_table_name, primary_key_value)
    SELECT tracked_table_identifier, old_table.id
    FROM old_table;

    RETURN NULL;
  END IF;

  -- Route every row to each target it carries a sharding key value for
  FOR target_table, target_column, source_column IN
    SELECT value ->> 'table', value ->> 'column', value ->> 'source'
    FROM jsonb_array_elements(targets)
  LOOP
    EXECUTE format(
      'INSERT INTO %I (fully_qualified_table_name, primary_key_value, %I)
       SELECT $1, old_table.id, old_table.%I
       FROM old_table
       WHERE old_table.%I IS NOT NULL',
       target_table, target_column, source_column, source_column
     )
    USING tracked_table_identifier;
  END LOOP;

  -- Rows carrying no sharding key value at all stay cell local. This filter is the exact
  -- complement of the loop's, so every deleted row lands in one branch or the other
  SELECT string_agg(format('old_table.%I IS NULL', value ->> 'source'), ' AND ')
  INTO cell_local_filter
  FROM jsonb_array_elements(targets);

  EXECUTE format(
    'INSERT INTO loose_foreign_keys_deleted_records
     (fully_qualified_table_name, primary_key_value)
     SELECT $1, old_table.id
     FROM old_table
     WHERE %s',
     cell_local_filter
   )
  USING tracked_table_identifier;

  RETURN NULL;
END
$_$;


--
-- Name: insert_into_loose_foreign_keys_deleted_records_override_table(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.insert_into_loose_foreign_keys_deleted_records_override_table() RETURNS trigger
    LANGUAGE plpgsql
    AS $_$
DECLARE
  parent_table_name TEXT := TG_ARGV[0];
  targets JSONB := CASE WHEN TG_NARGS > 1 THEN TG_ARGV[1]::jsonb ELSE '[]'::jsonb END;
  tracked_table_identifier TEXT := current_schema() || '.' || parent_table_name;
  target_table      TEXT;
  target_column     TEXT;
  source_column     TEXT;
  cell_local_filter TEXT;
BEGIN
  -- Cell local: no routing targets configured
  IF jsonb_array_length(targets) = 0 THEN
    INSERT INTO loose_foreign_keys_deleted_records
    (fully_qualified_table_name, primary_key_value)
    SELECT tracked_table_identifier, old_table.id
    FROM old_table;

    RETURN NULL;
  END IF;

  -- Route every row to each target it carries a sharding key value for
  FOR target_table, target_column, source_column IN
    SELECT value ->> 'table', value ->> 'column', value ->> 'source'
    FROM jsonb_array_elements(targets)
  LOOP
    EXECUTE format(
      'INSERT INTO %I (fully_qualified_table_name, primary_key_value, %I)
       SELECT $1, old_table.id, old_table.%I
       FROM old_table
       WHERE old_table.%I IS NOT NULL',
       target_table, target_column, source_column, source_column
     )
    USING tracked_table_identifier;
  END LOOP;

  -- Rows carrying no sharding key value at all stay cell local. This filter is the exact
  -- complement of the loop's, so every deleted row lands in one branch or the other
  SELECT string_agg(format('old_table.%I IS NULL', value ->> 'source'), ' AND ')
  INTO cell_local_filter
  FROM jsonb_array_elements(targets);

  EXECUTE format(
    'INSERT INTO loose_foreign_keys_deleted_records
     (fully_qualified_table_name, primary_key_value)
     SELECT $1, old_table.id
     FROM old_table
     WHERE %s',
     cell_local_filter
   )
  USING tracked_table_identifier;

  RETURN NULL;
END
$_$;


--
-- Name: insert_namespaces_sync_event(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.insert_namespaces_sync_event() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
INSERT INTO namespaces_sync_events (namespace_id)
VALUES(COALESCE(NEW.id, OLD.id));
RETURN NULL;

END
$$;


--
-- Name: insert_or_update_vulnerability_reads(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.insert_or_update_vulnerability_reads() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
  severity smallint;
  state smallint;
  report_type smallint;
  resolved_on_default_branch boolean;
  present_on_default_branch boolean;
  has_issues boolean;
  has_merge_request boolean;
BEGIN
  IF (SELECT current_setting('vulnerability_management.dont_execute_db_trigger', true) = 'true') THEN
    RETURN NULL;
  END IF;

  IF (NEW.vulnerability_id IS NULL AND (TG_OP = 'INSERT' OR TG_OP = 'UPDATE')) THEN
    RETURN NULL;
  END IF;

  IF (TG_OP = 'UPDATE' AND OLD.vulnerability_id IS NOT NULL AND NEW.vulnerability_id IS NOT NULL) THEN
    RETURN NULL;
  END IF;

  SELECT
    vulnerabilities.severity, vulnerabilities.state, vulnerabilities.report_type, vulnerabilities.resolved_on_default_branch, vulnerabilities.present_on_default_branch
  INTO
    severity, state, report_type, resolved_on_default_branch, present_on_default_branch
  FROM
    vulnerabilities
  WHERE
    vulnerabilities.id = NEW.vulnerability_id;

  IF present_on_default_branch IS NOT true THEN
    RETURN NULL;
  END IF;

  SELECT
    EXISTS (SELECT 1 FROM vulnerability_issue_links WHERE vulnerability_issue_links.vulnerability_id = NEW.vulnerability_id)
  INTO
    has_issues;

  SELECT
    EXISTS (SELECT 1 FROM vulnerability_merge_request_links WHERE vulnerability_merge_request_links.vulnerability_id = NEW.vulnerability_id)
  INTO
    has_merge_request;

  INSERT INTO vulnerability_reads (vulnerability_id, project_id, scanner_id, report_type, severity, state, resolved_on_default_branch, uuid, location_image, cluster_agent_id, casted_cluster_agent_id, has_issues, has_merge_request)
    VALUES (NEW.vulnerability_id, NEW.project_id, NEW.scanner_id, report_type, severity, state, resolved_on_default_branch, NEW.uuid::uuid, NEW.location->>'image', NEW.location->'kubernetes_resource'->>'agent_id', CAST(NEW.location->'kubernetes_resource'->>'agent_id' AS bigint), has_issues, has_merge_request)
    ON CONFLICT(vulnerability_id) DO NOTHING;
  RETURN NULL;
END
$$;


--
-- Name: insert_projects_sync_event(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.insert_projects_sync_event() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
INSERT INTO projects_sync_events (project_id)
VALUES(COALESCE(NEW.id, OLD.id));
RETURN NULL;

END
$$;


--
-- Name: insert_vulnerability_reads_from_vulnerability(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.insert_vulnerability_reads_from_vulnerability() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
  scanner_id bigint;
  uuid uuid;
  location_image text;
  cluster_agent_id text;
  casted_cluster_agent_id bigint;
  has_issues boolean;
  has_merge_request boolean;
BEGIN
  IF (SELECT current_setting('vulnerability_management.dont_execute_db_trigger', true) = 'true') THEN
    RETURN NULL;
  END IF;

  SELECT
    v_o.scanner_id, v_o.uuid, v_o.location->>'image', v_o.location->'kubernetes_resource'->>'agent_id', CAST(v_o.location->'kubernetes_resource'->>'agent_id' AS bigint)
  INTO
    scanner_id, uuid, location_image, cluster_agent_id, casted_cluster_agent_id
  FROM
    vulnerability_occurrences v_o
  WHERE
    v_o.vulnerability_id = NEW.id
  LIMIT 1;

  SELECT
    EXISTS (SELECT 1 FROM vulnerability_issue_links WHERE vulnerability_issue_links.vulnerability_id = NEW.id)
  INTO
    has_issues;

  SELECT
    EXISTS (SELECT 1 FROM vulnerability_merge_request_links WHERE vulnerability_merge_request_links.vulnerability_id = NEW.id)
  INTO
    has_merge_request;

  INSERT INTO vulnerability_reads (vulnerability_id, project_id, scanner_id, report_type, severity, state, resolved_on_default_branch, uuid, location_image, cluster_agent_id, casted_cluster_agent_id, has_issues, has_merge_request)
    VALUES (NEW.id, NEW.project_id, scanner_id, NEW.report_type, NEW.severity, NEW.state, NEW.resolved_on_default_branch, uuid::uuid, location_image, cluster_agent_id, casted_cluster_agent_id, has_issues, has_merge_request)
    ON CONFLICT(vulnerability_id) DO NOTHING;
  RETURN NULL;
END
$$;


--
-- Name: mark_geo_ci_job_artifact_verification_summary_dirty(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.mark_geo_ci_job_artifact_verification_summary_dirty() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
  v_bucket_number integer;
  v_id bigint;
BEGIN
  IF TG_OP = 'DELETE' THEN
    v_id := OLD.job_artifact_id;
  ELSE
    v_id := NEW.job_artifact_id;
  END IF;

  v_bucket_number := v_id % 100000;

  INSERT INTO geo_ci_job_artifact_verification_summaries
    (bucket_number, state, state_changed_at, created_at, updated_at)
  VALUES
    (v_bucket_number, 1, NOW(), NOW(), NOW())
  ON CONFLICT (bucket_number)
  DO UPDATE SET
    state = 1,
    state_changed_at = NOW(),
    updated_at = NOW()
  WHERE geo_ci_job_artifact_verification_summaries.state != 2;

  RETURN NULL;
END;
$$;


--
-- Name: merge_request_diffs_sync_bytea_sha_on_insert(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.merge_request_diffs_sync_bytea_sha_on_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW.base_commit_sha IS NOT NULL THEN
  NEW.base_commit_sha_bytea := decode(NEW.base_commit_sha, 'hex');
ELSIF NEW.base_commit_sha_bytea IS NOT NULL THEN
  NEW.base_commit_sha := encode(NEW.base_commit_sha_bytea, 'hex');
END IF;

IF NEW.start_commit_sha IS NOT NULL THEN
  NEW.start_commit_sha_bytea := decode(NEW.start_commit_sha, 'hex');
ELSIF NEW.start_commit_sha_bytea IS NOT NULL THEN
  NEW.start_commit_sha := encode(NEW.start_commit_sha_bytea, 'hex');
END IF;

IF NEW.head_commit_sha IS NOT NULL THEN
  NEW.head_commit_sha_bytea := decode(NEW.head_commit_sha, 'hex');
ELSIF NEW.head_commit_sha_bytea IS NOT NULL THEN
  NEW.head_commit_sha := encode(NEW.head_commit_sha_bytea, 'hex');
END IF;

RETURN NEW;

END
$$;


--
-- Name: merge_request_diffs_sync_bytea_sha_on_update(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.merge_request_diffs_sync_bytea_sha_on_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW.base_commit_sha IS DISTINCT FROM OLD.base_commit_sha THEN
  NEW.base_commit_sha_bytea := decode(NEW.base_commit_sha, 'hex');
ELSIF NEW.base_commit_sha_bytea IS DISTINCT FROM OLD.base_commit_sha_bytea THEN
  NEW.base_commit_sha := encode(NEW.base_commit_sha_bytea, 'hex');
END IF;

IF NEW.start_commit_sha IS DISTINCT FROM OLD.start_commit_sha THEN
  NEW.start_commit_sha_bytea := decode(NEW.start_commit_sha, 'hex');
ELSIF NEW.start_commit_sha_bytea IS DISTINCT FROM OLD.start_commit_sha_bytea THEN
  NEW.start_commit_sha := encode(NEW.start_commit_sha_bytea, 'hex');
END IF;

IF NEW.head_commit_sha IS DISTINCT FROM OLD.head_commit_sha THEN
  NEW.head_commit_sha_bytea := decode(NEW.head_commit_sha, 'hex');
ELSIF NEW.head_commit_sha_bytea IS DISTINCT FROM OLD.head_commit_sha_bytea THEN
  NEW.head_commit_sha := encode(NEW.head_commit_sha_bytea, 'hex');
END IF;

RETURN NEW;

END
$$;


--
-- Name: next_traversal_ids_sibling(bigint[]); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.next_traversal_ids_sibling(traversal_ids bigint[]) RETURNS bigint[]
    LANGUAGE plpgsql IMMUTABLE STRICT
    AS $$
BEGIN
  return traversal_ids[1:array_length(traversal_ids, 1)-1] ||
  ARRAY[traversal_ids[array_length(traversal_ids, 1)]+1];
END;
$$;


--
-- Name: nullify_merge_request_metrics_build_data(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.nullify_merge_request_metrics_build_data() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF (OLD.pipeline_id IS NOT NULL) AND (NEW.pipeline_id IS NULL) THEN
  NEW.latest_build_started_at = NULL;
  NEW.latest_build_finished_at = NULL;
END IF;
RETURN NEW;

END
$$;


--
-- Name: postgres_index_bloat_estimate(name, name); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.postgres_index_bloat_estimate(p_schema name, p_idxname name) RETURNS bigint
    LANGUAGE sql STABLE PARALLEL SAFE
    AS $$
    SELECT
        CASE
            WHEN ci.relpages::double precision > bloat.est_pages_ff
                THEN c.bs::double precision * (ci.relpages::double precision - bloat.est_pages_ff)
            ELSE 0::double precision
        END::bigint AS bloat_size_bytes
    FROM pg_class ci
    JOIN pg_index i      ON i.indexrelid = ci.oid
    JOIN pg_class ct     ON ct.oid = i.indrelid
    JOIN pg_namespace n  ON n.oid = ct.relnamespace

    CROSS JOIN LATERAL (
        SELECT
            current_setting('block_size')::numeric AS bs,
            CASE
                WHEN version() ~ 'mingw32'::text
                  OR version() ~ '64-bit|x86_64|ppc64|ia64|amd64'::text THEN 8
                ELSE 4
            END AS maxalign,
            24 AS pagehdr,
            16 AS pageopqdata,
            COALESCE(
                "substring"(array_to_string(ci.reloptions, ' '::text),
                            'fillfactor=([0-9]+)'::text)::smallint::integer,
                90
            ) AS fillfactor
    ) c

    CROSS JOIN LATERAL (
        SELECT
            max(CASE WHEN COALESCE(a1.atttypid, a2.atttypid) = 'name'::regtype::oid
                     THEN 1 ELSE 0 END) > 0 AS is_na,
            CASE WHEN max(COALESCE(s_t.null_frac, s_x.null_frac, 0::real)) = 0::double precision
                 THEN 2
                 ELSE 2 + (32 + 8 - 1) / 8
            END AS index_tuple_hdr_bm,
            sum((1::double precision - COALESCE(s_t.null_frac, s_x.null_frac, 0::real))
                * COALESCE(s_t.avg_width, s_x.avg_width, 1024)::double precision) AS nulldatawidth
        FROM generate_series(1, i.indnatts::integer) AS gs(attpos)
        LEFT JOIN pg_attribute a1
               ON (string_to_array(textin(int2vectorout(i.indkey)), ' '::text)::integer[])[gs.attpos] <> 0
              AND a1.attrelid = i.indrelid
              AND a1.attnum   = (string_to_array(textin(int2vectorout(i.indkey)), ' '::text)::integer[])[gs.attpos]
        LEFT JOIN pg_attribute a2
               ON (string_to_array(textin(int2vectorout(i.indkey)), ' '::text)::integer[])[gs.attpos] = 0
              AND a2.attrelid = ci.oid
              AND a2.attnum   = gs.attpos
        LEFT JOIN pg_stats s_t
               ON s_t.schemaname = n.nspname
              AND s_t.tablename  = ct.relname
              AND s_t.attname    = a1.attname
        LEFT JOIN pg_stats s_x
               ON s_x.schemaname = n.nspname
              AND s_x.tablename  = ci.relname
              AND s_x.attname    = a2.attname
    ) agg

    CROSS JOIN LATERAL (
        SELECT
            (
                (agg.index_tuple_hdr_bm + c.maxalign
                 - CASE WHEN (agg.index_tuple_hdr_bm % c.maxalign) = 0 THEN c.maxalign
                        ELSE agg.index_tuple_hdr_bm % c.maxalign END)::double precision
              + agg.nulldatawidth
              + c.maxalign::double precision
              - CASE WHEN agg.nulldatawidth = 0::double precision THEN 0
                     WHEN (agg.nulldatawidth::integer % c.maxalign) = 0 THEN c.maxalign
                     ELSE agg.nulldatawidth::integer % c.maxalign END::double precision
            )::numeric AS nulldatahdrwidth
    ) hdr

    CROSS JOIN LATERAL (
        SELECT
            COALESCE(
                1::double precision + ceil(
                    ci.reltuples / floor(
                        ((c.bs - c.pageopqdata::numeric - c.pagehdr::numeric) * c.fillfactor::numeric)::double precision
                        / (100::double precision * (4::numeric + hdr.nulldatahdrwidth)::double precision)
                    )
                ),
                0::double precision
            ) AS est_pages_ff,
            agg.is_na
    ) bloat

    WHERE ci.relkind = 'i'
      AND ci.relname = p_idxname
      AND n.nspname  = p_schema
      AND ci.relam   = (SELECT oid FROM pg_am WHERE amname = 'btree'::name)
      AND ci.relpages > 0
      AND NOT bloat.is_na;
$$;


--
-- Name: postgres_pg_stat_activity_autovacuum(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.postgres_pg_stat_activity_autovacuum() RETURNS TABLE(query text, query_start timestamp with time zone)
    LANGUAGE sql SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'pg_temp'
    AS $$
  SELECT query, query_start
  FROM pg_stat_activity
  WHERE datname = current_database()
    AND state = 'active'
    AND backend_type = 'autovacuum worker'
$$;


--
-- Name: prevent_delete_of_default_organization(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.prevent_delete_of_default_organization() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF OLD.id = 1 THEN
  RAISE EXCEPTION 'Deletion of the default Organization is not allowed.';
END IF;
RETURN OLD;

END
$$;


--
-- Name: repair_dual_sharding_key_on_notes(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.repair_dual_sharding_key_on_notes() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
NEW.namespace_id := NULL;

RETURN NEW;

END
$$;


--
-- Name: set_has_external_issue_tracker(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.set_has_external_issue_tracker() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
UPDATE projects SET has_external_issue_tracker = (
  EXISTS
  (
    SELECT 1
    FROM integrations
    WHERE project_id = COALESCE(NEW.project_id, OLD.project_id)
      AND active = TRUE
      AND category = 'issue_tracker'
  )
)
WHERE projects.id = COALESCE(NEW.project_id, OLD.project_id);
RETURN NULL;

END
$$;


--
-- Name: set_has_external_wiki(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.set_has_external_wiki() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
UPDATE projects SET has_external_wiki = COALESCE(NEW.active, FALSE)
WHERE projects.id = COALESCE(NEW.project_id, OLD.project_id);
RETURN NULL;

END
$$;


--
-- Name: set_has_issues_on_vulnerability_reads(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.set_has_issues_on_vulnerability_reads() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  IF (SELECT current_setting('vulnerability_management.dont_execute_db_trigger', true) = 'true') THEN
    RETURN NULL;
  END IF;

  UPDATE
    vulnerability_reads
  SET
    has_issues = true
  WHERE
    vulnerability_id = NEW.vulnerability_id AND has_issues IS FALSE;
  RETURN NULL;
END
$$;


--
-- Name: set_has_merge_request_on_vulnerability_reads(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.set_has_merge_request_on_vulnerability_reads() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  IF (SELECT current_setting('vulnerability_management.dont_execute_db_trigger', true) = 'true') THEN
    RETURN NULL;
  END IF;

  UPDATE
    vulnerability_reads
  SET
    has_merge_request = true
  WHERE
    vulnerability_id = NEW.vulnerability_id AND has_merge_request IS FALSE;
  RETURN NULL;
END
$$;


--
-- Name: sync_issues_dates_with_work_item_dates_sources(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.sync_issues_dates_with_work_item_dates_sources() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
UPDATE
  issues
SET
  start_date = NEW.start_date,
  due_date = NEW.due_date
WHERE
  issues.id = NEW.issue_id;

RETURN NULL;

END
$$;


--
-- Name: sync_packages_composer_with_composer_metadata(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.sync_packages_composer_with_composer_metadata() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
UPDATE "packages_composer_packages"
    SET target_sha = NEW.target_sha,
        composer_json = NEW.composer_json,
        version_cache_sha = NEW.version_cache_sha
    WHERE id = NEW.package_id;
RETURN NULL;

END
$$;


--
-- Name: sync_packages_composer_with_packages(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.sync_packages_composer_with_packages() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF (COALESCE(NEW.package_type, OLD.package_type) = 6) THEN
  IF (TG_OP = 'INSERT') THEN
    INSERT INTO "packages_composer_packages" (id, project_id, created_at, updated_at, name, version, creator_id, status, last_downloaded_at, status_message)
      VALUES (NEW.id, NEW.project_id, NEW.created_at, NEW.updated_at, NEW.name, NEW.version, NEW.creator_id, NEW.status, NEW.last_downloaded_at, NEW.status_message);
  ELSIF (TG_OP = 'UPDATE') THEN
    UPDATE "packages_composer_packages"
        SET project_id = NEW.project_id,
            updated_at = NEW.updated_at,
            name = NEW.name,
            version = NEW.version,
            creator_id = NEW.creator_id,
            status = NEW.status,
            last_downloaded_at = NEW.last_downloaded_at,
            status_message = NEW.status_message
        WHERE id = OLD.id;
  ELSIF (TG_OP = 'DELETE') THEN
    DELETE FROM "packages_composer_packages" WHERE id = OLD.id;
  END IF;
END IF;
RETURN NULL;

END
$$;


--
-- Name: sync_project_authorizations_to_migration_table(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.sync_project_authorizations_to_migration_table() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF (TG_OP = 'INSERT' OR TG_OP = 'UPDATE') THEN
  INSERT INTO project_authorizations_for_migration (project_id, user_id, access_level)
  VALUES (NEW.project_id, NEW.user_id, NEW.access_level::smallint)
  ON CONFLICT (project_id, user_id) DO UPDATE
    SET access_level = NEW.access_level::smallint;
  RETURN NEW;

ELSIF (TG_OP = 'DELETE') THEN
  WITH remaining AS (
    SELECT project_id, user_id, MIN(access_level)::smallint AS access_level
    FROM project_authorizations
    WHERE project_id = OLD.project_id AND user_id = OLD.user_id
    GROUP BY project_id, user_id
  ), upsert AS (
    INSERT INTO project_authorizations_for_migration (project_id, user_id, access_level)
    SELECT project_id, user_id, access_level
    FROM remaining
    ON CONFLICT (project_id, user_id) DO UPDATE
      SET access_level = EXCLUDED.access_level
  )
  DELETE FROM project_authorizations_for_migration
  WHERE project_id = OLD.project_id AND user_id = OLD.user_id
    AND NOT EXISTS (SELECT 1 FROM remaining);
  RETURN OLD;
END IF;

RETURN NULL;

END
$$;


--
-- Name: sync_redirect_routes_namespace_id(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.sync_redirect_routes_namespace_id() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."source_type" = 'Namespace' THEN
  NEW."namespace_id" = NEW."source_id";
ELSIF NEW."source_type" = 'Project' THEN
  NEW."namespace_id" = (SELECT project_namespace_id FROM projects WHERE id = NEW.source_id);
END IF;

RETURN NEW;

END
$$;


--
-- Name: sync_sharding_key_with_notes_table(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.sync_sharding_key_with_notes_table() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
  note_project_id BIGINT;
  note_namespace_id BIGINT;
BEGIN
  IF NEW."note_id" IS NULL OR NEW."namespace_id" IS NOT NULL THEN
    RETURN NEW;
  END IF;

  SELECT "project_id", "namespace_id"
  INTO note_project_id, note_namespace_id
  FROM "notes"
  WHERE "id" = NEW."note_id";

  IF note_project_id IS NOT NULL THEN
    SELECT "project_namespace_id" FROM "projects"
    INTO NEW."namespace_id" WHERE "projects"."id" = note_project_id;
  ELSE
    NEW."namespace_id" := note_namespace_id;
  END IF;

  RETURN NEW;
END
$$;


--
-- Name: sync_user_id_from_gpg_keys_table(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.sync_user_id_from_gpg_keys_table() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  IF NEW."gpg_key_id" IS NULL OR NEW."user_id" IS NOT NULL THEN
    RETURN NEW;
  END IF;

  SELECT "user_id"
  INTO NEW."user_id"
  FROM "gpg_keys"
  WHERE "id" = NEW."gpg_key_id";

  RETURN NEW;
END
$$;


--
-- Name: sync_work_item_positions_from_issues(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.sync_work_item_positions_from_issues() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
INSERT INTO work_item_positions (
  work_item_id,
  namespace_id,
  relative_positioning_namespace_id,
  relative_position,
  created_at,
  updated_at
)
VALUES (
  NEW.id,
  NEW.namespace_id,
  (
    SELECT CASE
      WHEN p.type = 'User' OR p.type IS NULL THEN n.id
      ELSE COALESCE(n.traversal_ids[1], n.id)
    END
    FROM namespaces n
    LEFT JOIN namespaces p ON p.id = n.parent_id
    WHERE n.id = NEW.namespace_id
  ),
  NEW.relative_position,
  NOW(),
  NOW()
)
ON CONFLICT (work_item_id)
DO UPDATE SET
  relative_position = EXCLUDED.relative_position,
  namespace_id = EXCLUDED.namespace_id,
  relative_positioning_namespace_id = EXCLUDED.relative_positioning_namespace_id,
  updated_at = NOW();
RETURN NULL;

END
$$;


--
-- Name: sync_work_item_transitions_from_issues(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.sync_work_item_transitions_from_issues() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
INSERT INTO work_item_transitions (
  work_item_id,
  namespace_id,
  moved_to_id,
  duplicated_to_id,
  promoted_to_epic_id
)
VALUES (
  NEW.id,
  NEW.namespace_id,
  NEW.moved_to_id,
  NEW.duplicated_to_id,
  NEW.promoted_to_epic_id
)
ON CONFLICT (work_item_id)
DO UPDATE SET
  moved_to_id = EXCLUDED.moved_to_id,
  duplicated_to_id = EXCLUDED.duplicated_to_id,
  promoted_to_epic_id = EXCLUDED.promoted_to_epic_id,
  namespace_id = EXCLUDED.namespace_id;
RETURN NULL;

END
$$;


--
-- Name: table_sync_function_0992e728d3_delete(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.table_sync_function_0992e728d3_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
DELETE FROM merge_request_diff_commits_b5377a7a34
WHERE (merge_request_diff_id, relative_order, project_id) IN (
  SELECT
    old_table.merge_request_diff_id,
    old_table.relative_order,
    old_table.project_id
  FROM old_table
  WHERE old_table.project_id IS NOT NULL
);

RETURN NULL;

END
$$;


--
-- Name: table_sync_function_0992e728d3_insert(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.table_sync_function_0992e728d3_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
INSERT INTO merge_request_diff_commits_b5377a7a34
  (merge_request_commits_metadata_id, project_id, merge_request_diff_id, relative_order)
SELECT
  new_table.merge_request_commits_metadata_id,
  new_table.project_id,
  new_table.merge_request_diff_id,
  new_table.relative_order
FROM new_table
WHERE new_table.merge_request_commits_metadata_id IS NOT NULL
  AND new_table.project_id IS NOT NULL;

RETURN NULL;

END
$$;


--
-- Name: table_sync_function_3f39f64fc3_reverse(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.table_sync_function_3f39f64fc3_reverse() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF (TG_OP = 'DELETE') THEN
  DELETE FROM merge_request_diff_files_archived
  WHERE "merge_request_diff_id" = OLD."merge_request_diff_id"
    AND "relative_order" = OLD."relative_order";
ELSIF (TG_OP = 'UPDATE') THEN
  IF NEW."merge_request_diff_id" <= 2147483647 THEN
    UPDATE merge_request_diff_files_archived
    SET "new_file" = NEW."new_file",
      "renamed_file" = NEW."renamed_file",
      "deleted_file" = NEW."deleted_file",
      "too_large" = NEW."too_large",
      "a_mode" = NEW."a_mode",
      "b_mode" = NEW."b_mode",
      "new_path" = NULLIF(NEW."new_path", NEW."old_path"),
      "old_path" = NEW."old_path",
      "diff" = NEW."diff",
      "binary" = NEW."binary",
      "external_diff_offset" = NEW."external_diff_offset",
      "external_diff_size" = NEW."external_diff_size",
      "generated" = NEW."generated",
      "encoded_file_path" = NEW."encoded_file_path",
      "project_id" = NEW."project_id"
    WHERE merge_request_diff_files_archived."merge_request_diff_id" = NEW."merge_request_diff_id"
      AND merge_request_diff_files_archived."relative_order" = NEW."relative_order";
  END IF;
ELSIF (TG_OP = 'INSERT') THEN
  IF NEW."merge_request_diff_id" <= 2147483647 THEN
    INSERT INTO merge_request_diff_files_archived (
      "merge_request_diff_id",
      "relative_order",
      "new_file",
      "renamed_file",
      "deleted_file",
      "too_large",
      "a_mode",
      "b_mode",
      "new_path",
      "old_path",
      "diff",
      "binary",
      "external_diff_offset",
      "external_diff_size",
      "generated",
      "encoded_file_path",
      "project_id"
    )
    VALUES (
      NEW."merge_request_diff_id",
      NEW."relative_order",
      NEW."new_file",
      NEW."renamed_file",
      NEW."deleted_file",
      NEW."too_large",
      NEW."a_mode",
      NEW."b_mode",
      NULLIF(NEW."new_path", NEW."old_path"),
      NEW."old_path",
      NEW."diff",
      NEW."binary",
      NEW."external_diff_offset",
      NEW."external_diff_size",
      NEW."generated",
      NEW."encoded_file_path",
      NEW."project_id"
    )
    ON CONFLICT ("merge_request_diff_id", "relative_order") DO NOTHING;
  END IF;
END IF;

RETURN NULL;

END
$$;


--
-- Name: timestamp_coalesce(timestamp with time zone, anyelement); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.timestamp_coalesce(t1 timestamp with time zone, t2 anyelement) RETURNS timestamp without time zone
    LANGUAGE plpgsql IMMUTABLE
    AS $$
BEGIN
  RETURN COALESCE(t1::TIMESTAMP, t2);
END;
$$;


--
-- Name: todos_sharding_key(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.todos_sharding_key() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  IF num_nonnulls(NEW.organization_id, NEW.group_id, NEW.project_id) != 1 THEN
    IF NEW.project_id IS NOT NULL THEN
      NEW.organization_id := NULL;
      NEW.group_id := NULL;
    ELSIF NEW.group_id IS NOT NULL THEN
      NEW.organization_id := NULL;
      NEW.project_id := NULL;
    ELSE
      SELECT "organization_id", NULL, NULL
      INTO NEW."organization_id", NEW."group_id", NEW."project_id"
      FROM "users"
      WHERE "users"."id" = NEW."user_id";
    END IF;
  END IF;

  RETURN NEW;
END
$$;


--
-- Name: trigger_009314eae986(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_009314eae986() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."protected_branch_project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."protected_branch_project_id"
  FROM "protected_branches"
  WHERE "protected_branches"."id" = NEW."protected_branch_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_01b3fc052119(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_01b3fc052119() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "target_project_id"
  INTO NEW."project_id"
  FROM "merge_requests"
  WHERE "merge_requests"."id" = NEW."merge_request_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_02450faab875(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_02450faab875() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "vulnerability_occurrences"
  WHERE "vulnerability_occurrences"."id" = NEW."occurrence_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_038fe84feff7(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_038fe84feff7() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "target_project_id"
  INTO NEW."project_id"
  FROM "merge_requests"
  WHERE "merge_requests"."id" = NEW."merge_request_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_05cc4448a8aa(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_05cc4448a8aa() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."protected_branch_namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."protected_branch_namespace_id"
  FROM "protected_branches"
  WHERE "protected_branches"."id" = NEW."protected_branch_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_05ce163deddf(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_05ce163deddf() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "target_project_id"
  INTO NEW."project_id"
  FROM "merge_requests"
  WHERE "merge_requests"."id" = NEW."merge_request_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_08ab48583e86(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_08ab48583e86() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."organization_id" IS NULL THEN
  SELECT "organization_id"
  INTO NEW."organization_id"
  FROM "user_uploads"
  WHERE "user_uploads"."id" = NEW."user_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_0a1b0adcf686(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_0a1b0adcf686() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "packages_debian_project_distributions"
  WHERE "packages_debian_project_distributions"."id" = NEW."distribution_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_0a29d4d42b62(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_0a29d4d42b62() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "approval_project_rules"
  WHERE "approval_project_rules"."id" = NEW."approval_project_rule_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_0aea02e5a699(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_0aea02e5a699() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."protected_branch_project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."protected_branch_project_id"
  FROM "protected_branches"
  WHERE "protected_branches"."id" = NEW."protected_branch_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_0af180e1ec89(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_0af180e1ec89() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "packages_debian_project_components"
  WHERE "packages_debian_project_components"."id" = NEW."component_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_0b497498ae51(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_0b497498ae51() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "import_export_upload_uploads"
  WHERE "import_export_upload_uploads"."id" = NEW."import_export_upload_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_0c326daf67cf(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_0c326daf67cf() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "group_id"
  INTO NEW."namespace_id"
  FROM "analytics_cycle_analytics_group_value_streams"
  WHERE "analytics_cycle_analytics_group_value_streams"."id" = NEW."value_stream_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_0d96daa4d734(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_0d96daa4d734() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."group_id" IS NULL THEN
  SELECT "group_id"
  INTO NEW."group_id"
  FROM "bulk_import_exports"
  WHERE "bulk_import_exports"."id" = NEW."export_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_0da002390fdc(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_0da002390fdc() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "operations_feature_flags"
  WHERE "operations_feature_flags"."id" = NEW."feature_flag_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_0ddb594934c9(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_0ddb594934c9() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "incident_management_oncall_rotations"
  WHERE "incident_management_oncall_rotations"."id" = NEW."oncall_rotation_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_0e13f214e504(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_0e13f214e504() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "target_project_id"
  INTO NEW."project_id"
  FROM "merge_requests"
  WHERE "merge_requests"."id" = NEW."merge_request_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_0f38e5af9adf(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_0f38e5af9adf() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "ml_candidates"
  WHERE "ml_candidates"."id" = NEW."candidate_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_13d4aa8fe3dd(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_13d4aa8fe3dd() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "target_project_id"
  INTO NEW."project_id"
  FROM "merge_requests"
  WHERE "merge_requests"."id" = NEW."merge_request_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_14a39509be0a(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_14a39509be0a() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "packages_packages"
  WHERE "packages_packages"."id" = NEW."package_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_1513378d715d(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_1513378d715d() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issues"
  WHERE "issues"."id" = NEW."issue_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_158ac875f254(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_158ac875f254() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."group_id" IS NULL THEN
  SELECT "group_id"
  INTO NEW."group_id"
  FROM "approval_group_rules"
  WHERE "approval_group_rules"."id" = NEW."approval_group_rule_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_16bb23b09f5d(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_16bb23b09f5d() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."organization_id" IS NULL THEN
  SELECT "organization_id"
  INTO NEW."organization_id"
  FROM "vulnerability_export_part_uploads"
  WHERE "vulnerability_export_part_uploads"."id" = NEW."vulnerability_export_part_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_174b23fa3dfb(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_174b23fa3dfb() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "approval_project_rules"
  WHERE "approval_project_rules"."id" = NEW."approval_project_rule_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_1825cdc71779(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_1825cdc71779() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."organization_id" IS NULL THEN
  SELECT "organization_id"
  INTO NEW."organization_id"
  FROM "organization_details"
  WHERE "organization_details"."organization_id" = NEW."model_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_18bc439a6741(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_18bc439a6741() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "packages_packages"
  WHERE "packages_packages"."id" = NEW."package_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_1996c9e5bea0(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_1996c9e5bea0() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."organization_id" IS NULL THEN
  SELECT "organization_id"
  INTO NEW."organization_id"
  FROM "abuse_reports"
  WHERE "abuse_reports"."id" = NEW."abuse_report_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_199f655f86af(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_199f655f86af() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "ci_pipeline_artifacts"
  WHERE "ci_pipeline_artifacts"."id" = NEW."pipeline_artifact_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_1a052e65e9d9(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_1a052e65e9d9() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "group_id"
  INTO NEW."namespace_id"
  FROM "import_export_uploads"
  WHERE "import_export_uploads"."id" = NEW."model_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_1a41d368edd5(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_1a41d368edd5() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "import_export_uploads"
  WHERE "import_export_uploads"."id" = NEW."model_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_1c0f1ca199a3(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_1c0f1ca199a3() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "ci_resource_groups"
  WHERE "ci_resource_groups"."id" = NEW."resource_group_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_1e61c7e33a36(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_1e61c7e33a36() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "vulnerability_archive_export_uploads"
  WHERE "vulnerability_archive_export_uploads"."id" = NEW."vulnerability_archive_export_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_1e75dc6149d6(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_1e75dc6149d6() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "import_export_upload_uploads"
  WHERE "import_export_upload_uploads"."id" = NEW."import_export_upload_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_1ed40f4d5f4e(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_1ed40f4d5f4e() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "packages_packages"
  WHERE "packages_packages"."id" = NEW."package_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_1eda1bc6ef53(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_1eda1bc6ef53() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "merge_request_diffs"
  WHERE "merge_request_diffs"."id" = NEW."merge_request_diff_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_1f57c71a69fb(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_1f57c71a69fb() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."snippet_organization_id" IS NULL THEN
  SELECT "organization_id"
  INTO NEW."snippet_organization_id"
  FROM "snippets"
  WHERE "snippets"."id" = NEW."snippet_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_206cbe2dc1a2(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_206cbe2dc1a2() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "packages_packages"
  WHERE "packages_packages"."id" = NEW."package_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_207005e8e995(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_207005e8e995() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "operations_feature_flags"
  WHERE "operations_feature_flags"."id" = NEW."feature_flag_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_218433b4faa5(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_218433b4faa5() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "packages_package_files"
  WHERE "packages_package_files"."id" = NEW."package_file_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_219952df8fc4(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_219952df8fc4() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "target_project_id"
  INTO NEW."project_id"
  FROM "merge_requests"
  WHERE "merge_requests"."id" = NEW."blocking_merge_request_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_238f37f25bb2(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_238f37f25bb2() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."group_id" IS NULL THEN
  SELECT "group_id"
  INTO NEW."group_id"
  FROM "boards_epic_lists"
  WHERE "boards_epic_lists"."id" = NEW."epic_list_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_243aecba8654(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_243aecba8654() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "dast_site_profiles"
  WHERE "dast_site_profiles"."id" = NEW."dast_site_profile_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_248cafd363ff(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_248cafd363ff() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "packages_packages"
  WHERE "packages_packages"."id" = NEW."package_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_2514245c7fc5(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_2514245c7fc5() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "dast_site_profiles"
  WHERE "dast_site_profiles"."id" = NEW."dast_site_profile_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_25ba78722e56(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_25ba78722e56() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."organization_id" IS NULL THEN
  SELECT "organization_id"
  INTO NEW."organization_id"
  FROM "users"
  WHERE "users"."id" = NEW."model_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_25c44c30884f(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_25c44c30884f() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issues"
  WHERE "issues"."id" = NEW."work_item_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_25d35f02ab55(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_25d35f02ab55() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "ml_candidates"
  WHERE "ml_candidates"."id" = NEW."candidate_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_25fe4f7da510(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_25fe4f7da510() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "vulnerabilities"
  WHERE "vulnerabilities"."id" = NEW."vulnerability_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_29128c51c7c6(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_29128c51c7c6() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "dast_pre_scan_verifications"
  WHERE "dast_pre_scan_verifications"."id" = NEW."dast_pre_scan_verification_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_292097dea85c(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_292097dea85c() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "terraform_state_versions"
  WHERE "terraform_state_versions"."id" = NEW."terraform_state_version_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_2a550aba90e3(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_2a550aba90e3() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "vulnerability_remediation_uploads"
  WHERE "vulnerability_remediation_uploads"."id" = NEW."vulnerability_remediation_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_2a994bb5629f(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_2a994bb5629f() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "alert_management_alerts"
  WHERE "alert_management_alerts"."id" = NEW."alert_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_2b8fdc9b4a4e(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_2b8fdc9b4a4e() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "ml_experiments"
  WHERE "ml_experiments"."id" = NEW."experiment_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_2cb7e7147818(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_2cb7e7147818() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT COALESCE("notes"."namespace_id", "projects"."project_namespace_id")
  INTO NEW."namespace_id"
  FROM "notes"
  LEFT JOIN "projects" ON "projects"."id" = "notes"."project_id"
  WHERE "notes"."id" = NEW."note_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_2dafd0d13605(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_2dafd0d13605() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "pages_domains"
  WHERE "pages_domains"."id" = NEW."pages_domain_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_2e4861e8640c(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_2e4861e8640c() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "packages_package_files"
  WHERE "packages_package_files"."id" = NEW."package_file_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_30209d0fba3e(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_30209d0fba3e() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "alert_management_alerts"
  WHERE "alert_management_alerts"."id" = NEW."alert_management_alert_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_309294c3b889(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_309294c3b889() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."snippet_project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."snippet_project_id"
  FROM "snippets"
  WHERE "snippets"."id" = NEW."snippet_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_31b1148083b3(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_31b1148083b3() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "bulk_import_export_upload_uploads"
  WHERE "bulk_import_export_upload_uploads"."id" = NEW."bulk_import_export_upload_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_3434b82e5e12(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_3434b82e5e12() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."organization_id" IS NULL THEN
  SELECT "organization_id"
  INTO NEW."organization_id"
  FROM "abuse_reports"
  WHERE "abuse_reports"."id" = NEW."model_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_363d0fd35f2c(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_363d0fd35f2c() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "packages_dependency_links"
  WHERE "packages_dependency_links"."id" = NEW."dependency_link_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_3691f9f6a69f(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_3691f9f6a69f() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "cluster_agents"
  WHERE "cluster_agents"."id" = NEW."cluster_agent_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_36cb404f9a02(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_36cb404f9a02() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."organization_id" IS NULL THEN
  SELECT "organization_id"
  INTO NEW."organization_id"
  FROM "bulk_import_entities"
  WHERE "bulk_import_entities"."id" = NEW."bulk_import_entity_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_388de55cd36c(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_388de55cd36c() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "p_ci_builds"
  WHERE "p_ci_builds"."id" = NEW."build_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_38b6d9d97935(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_38b6d9d97935() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "id"
  INTO NEW."project_id"
  FROM "projects"
  WHERE "projects"."id" = NEW."model_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_38bfee591e40(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_38bfee591e40() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."group_id" IS NULL THEN
  SELECT "group_id"
  INTO NEW."group_id"
  FROM "dependency_proxy_blobs"
  WHERE "dependency_proxy_blobs"."id" = NEW."dependency_proxy_blob_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_397d1b13068e(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_397d1b13068e() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issues"
  WHERE "issues"."id" = NEW."issue_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_39a0715793cf(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_39a0715793cf() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."organization_id" IS NULL THEN
  SELECT "organization_id"
  INTO NEW."organization_id"
  FROM "dependency_list_export_part_uploads"
  WHERE "dependency_list_export_part_uploads"."id" = NEW."dependency_list_export_part_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_3be1956babdb(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_3be1956babdb() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."snippet_organization_id" IS NULL THEN
  SELECT "organization_id"
  INTO NEW."snippet_organization_id"
  FROM "snippets"
  WHERE "snippets"."id" = NEW."snippet_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_3c1a5f58a668(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_3c1a5f58a668() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "incident_management_oncall_rotations"
  WHERE "incident_management_oncall_rotations"."id" = NEW."rotation_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_3d1a58344b29(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_3d1a58344b29() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "alert_management_alerts"
  WHERE "alert_management_alerts"."id" = NEW."alert_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_3e067fa9bfe3(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_3e067fa9bfe3() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "incident_management_timeline_event_tags"
  WHERE "incident_management_timeline_event_tags"."id" = NEW."timeline_event_tag_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_3f28a0bfdb16(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_3f28a0bfdb16() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "target_project_id"
  INTO NEW."project_id"
  FROM "merge_requests"
  WHERE "merge_requests"."id" = NEW."merge_request_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_3fe922f4db67(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_3fe922f4db67() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "vulnerabilities"
  WHERE "vulnerabilities"."id" = NEW."vulnerability_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_41eaf23bf547(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_41eaf23bf547() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "releases"
  WHERE "releases"."id" = NEW."release_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_43484cb41aca(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_43484cb41aca() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "project_wiki_repositories"
  WHERE "project_wiki_repositories"."id" = NEW."project_wiki_repository_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_442d030cfdfe(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_442d030cfdfe() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "id"
  INTO NEW."namespace_id"
  FROM "namespaces"
  WHERE "namespaces"."id" = NEW."model_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_44558add1625(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_44558add1625() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "target_project_id"
  INTO NEW."project_id"
  FROM "merge_requests"
  WHERE "merge_requests"."id" = NEW."merge_request_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_44ff19ad0ab2(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_44ff19ad0ab2() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "packages_packages"
  WHERE "packages_packages"."id" = NEW."package_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_468b8554e533(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_468b8554e533() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "vulnerability_scanners"
  WHERE "vulnerability_scanners"."id" = NEW."scanner_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_46ebe375f632(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_46ebe375f632() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issues"
  WHERE "issues"."id" = NEW."issue_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_47b402bdab5f(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_47b402bdab5f() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "bulk_import_exports"
  WHERE "bulk_import_exports"."id" = NEW."export_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_47b8922fa2f4(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_47b8922fa2f4() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."organization_id" IS NULL THEN
  SELECT "organization_id"
  INTO NEW."organization_id"
  FROM "organization_detail_uploads"
  WHERE "organization_detail_uploads"."id" = NEW."organization_detail_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_47c43d40f0d2(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_47c43d40f0d2() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "alert_management_alert_metric_images"
  WHERE "alert_management_alert_metric_images"."id" = NEW."model_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_489fffe04425(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_489fffe04425() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "packages_helm_metadata_caches"
  WHERE "packages_helm_metadata_caches"."id" = NEW."packages_helm_metadata_cache_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_49862b4b3035(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_49862b4b3035() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."group_id" IS NULL THEN
  SELECT "group_id"
  INTO NEW."group_id"
  FROM "approval_group_rules"
  WHERE "approval_group_rules"."id" = NEW."approval_group_rule_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_49b563d0130b(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_49b563d0130b() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "dast_scanner_profiles"
  WHERE "dast_scanner_profiles"."id" = NEW."dast_scanner_profile_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_49e070da6320(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_49e070da6320() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "packages_packages"
  WHERE "packages_packages"."id" = NEW."package_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_4ad9a52a6614(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_4ad9a52a6614() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "sbom_occurrences"
  WHERE "sbom_occurrences"."id" = NEW."sbom_occurrence_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_4b43790d717f(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_4b43790d717f() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."protected_environment_group_id" IS NULL THEN
  SELECT "group_id"
  INTO NEW."protected_environment_group_id"
  FROM "protected_environments"
  WHERE "protected_environments"."id" = NEW."protected_environment_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_4c320a13bc8d(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_4c320a13bc8d() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "security_orchestration_policy_configurations"
  WHERE "security_orchestration_policy_configurations"."id" = NEW."security_orchestration_policy_configuration_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_4cc5c3ac4d7f(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_4cc5c3ac4d7f() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "bulk_import_exports"
  WHERE "bulk_import_exports"."id" = NEW."export_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_4dc8ec48e038(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_4dc8ec48e038() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "issues"
  WHERE "issues"."id" = NEW."issue_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_4f1b6c76fdfc(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_4f1b6c76fdfc() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."organization_id" IS NULL THEN
  SELECT "organization_id"
  INTO NEW."organization_id"
  FROM "topics"
  WHERE "topics"."id" = NEW."model_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_4fc14aa830b1(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_4fc14aa830b1() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issues"
  WHERE "issues"."id" = NEW."work_item_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_54707c384ad7(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_54707c384ad7() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "security_orchestration_policy_configurations"
  WHERE "security_orchestration_policy_configurations"."id" = NEW."security_orchestration_policy_configuration_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_553243728f0d(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_553243728f0d() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "dependency_list_export_uploads"
  WHERE "dependency_list_export_uploads"."id" = NEW."dependency_list_export_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_5682f7f9cbc0(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_5682f7f9cbc0() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "packages_nuget_symbols"
  WHERE "packages_nuget_symbols"."id" = NEW."packages_nuget_symbol_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_56d49f4ed623(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_56d49f4ed623() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "workspaces"
  WHERE "workspaces"."id" = NEW."workspace_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_57ad2742ac16(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_57ad2742ac16() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "achievements"
  WHERE "achievements"."id" = NEW."achievement_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_57d53b2ab135(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_57d53b2ab135() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issues"
  WHERE "issues"."id" = NEW."issue_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_589db52d2d69(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_589db52d2d69() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issues"
  WHERE "issues"."id" = NEW."issue_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_5afaa56f3e0b(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_5afaa56f3e0b() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."organization_id" IS NULL THEN
  SELECT "organization_id"
  INTO NEW."organization_id"
  FROM "vulnerability_export_uploads"
  WHERE "vulnerability_export_uploads"."id" = NEW."vulnerability_export_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_5ca97b87ee30(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_5ca97b87ee30() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "target_project_id"
  INTO NEW."project_id"
  FROM "merge_requests"
  WHERE "merge_requests"."id" = NEW."merge_request_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_5cf44cd40f22(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_5cf44cd40f22() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "operations_strategies"
  WHERE "operations_strategies"."id" = NEW."strategy_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_5ed68c226e97(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_5ed68c226e97() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "approval_merge_request_rules"
  WHERE "approval_merge_request_rules"."id" = NEW."approval_merge_request_rule_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_5f6432d2dccc(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_5f6432d2dccc() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "operations_user_lists"
  WHERE "operations_user_lists"."id" = NEW."user_list_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_627949f72f05(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_627949f72f05() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "packages_packages"
  WHERE "packages_packages"."id" = NEW."package_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_632bcdfce430(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_632bcdfce430() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "dependency_list_export_uploads"
  WHERE "dependency_list_export_uploads"."id" = NEW."dependency_list_export_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_664594a3d0a7(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_664594a3d0a7() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "target_project_id"
  INTO NEW."project_id"
  FROM "merge_requests"
  WHERE "merge_requests"."id" = NEW."merge_request_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_67d0d39e2f41(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_67d0d39e2f41() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."uploaded_by_user_id" IS NULL THEN
  SELECT "user_id"
  INTO NEW."uploaded_by_user_id"
  FROM "user_permission_export_uploads"
  WHERE "user_permission_export_uploads"."id" = NEW."model_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_68435a54ee2b(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_68435a54ee2b() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "packages_debian_project_distributions"
  WHERE "packages_debian_project_distributions"."id" = NEW."distribution_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_6b658eff5ad3(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_6b658eff5ad3() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "slsa_attestations"
  WHERE "slsa_attestations"."id" = NEW."supply_chain_attestation_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_6bf50b363152(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_6bf50b363152() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "security_orchestration_policy_configurations"
  WHERE "security_orchestration_policy_configurations"."id" = NEW."policy_configuration_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_6c38ba395cc1(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_6c38ba395cc1() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "error_tracking_errors"
  WHERE "error_tracking_errors"."id" = NEW."error_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_6c4657b1b157(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_6c4657b1b157() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."organization_id" IS NULL THEN
  SELECT "organization_id"
  INTO NEW."organization_id"
  FROM "dependency_list_export_uploads"
  WHERE "dependency_list_export_uploads"."id" = NEW."dependency_list_export_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_6cdea9559242(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_6cdea9559242() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issues"
  WHERE "issues"."id" = NEW."source_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_6d6c79ce74e1(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_6d6c79ce74e1() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."protected_environment_project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."protected_environment_project_id"
  FROM "protected_environments"
  WHERE "protected_environments"."id" = NEW."protected_environment_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_6fc75a2395f3(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_6fc75a2395f3() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "packages_package_files"
  WHERE "packages_package_files"."id" = NEW."package_file_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_700f29b1312e(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_700f29b1312e() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "packages_packages"
  WHERE "packages_packages"."id" = NEW."package_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_70d3f0bba1de(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_70d3f0bba1de() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "security_orchestration_policy_configurations"
  WHERE "security_orchestration_policy_configurations"."id" = NEW."policy_configuration_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_738125833856(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_738125833856() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."organization_id" IS NULL THEN
  SELECT "organization_id"
  INTO NEW."organization_id"
  FROM "bulk_imports"
  WHERE "bulk_imports"."id" = NEW."bulk_import_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_740afa9807b8(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_740afa9807b8() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."organization_id" IS NULL THEN
  SELECT "organization_id"
  INTO NEW."organization_id"
  FROM "subscription_add_on_purchases"
  WHERE "subscription_add_on_purchases"."id" = NEW."add_on_purchase_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_744ab45ee5ac(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_744ab45ee5ac() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."protected_branch_namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."protected_branch_namespace_id"
  FROM "protected_branches"
  WHERE "protected_branches"."id" = NEW."protected_branch_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_7495f5e0efcb(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_7495f5e0efcb() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."snippet_project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."snippet_project_id"
  FROM "snippets"
  WHERE "snippets"."id" = NEW."snippet_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_77d9fbad5b12(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_77d9fbad5b12() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "packages_debian_project_distributions"
  WHERE "packages_debian_project_distributions"."id" = NEW."distribution_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_78c85ddc4031(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_78c85ddc4031() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issues"
  WHERE "issues"."id" = NEW."issue_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_7943cb549289(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_7943cb549289() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issues"
  WHERE "issues"."id" = NEW."issue_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_7a6d75e9eecd(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_7a6d75e9eecd() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "project_relation_exports"
  WHERE "project_relation_exports"."id" = NEW."project_relation_export_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_7a8b08eed782(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_7a8b08eed782() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."group_id" IS NULL THEN
  SELECT "group_id"
  INTO NEW."group_id"
  FROM "boards_epic_boards"
  WHERE "boards_epic_boards"."id" = NEW."epic_board_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_7b21c87a1f91(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_7b21c87a1f91() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "bulk_import_entities"
  WHERE "bulk_import_entities"."id" = NEW."bulk_import_entity_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_7b378a0c402b(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_7b378a0c402b() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issues"
  WHERE "issues"."id" = NEW."issue_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_7d206f446d34(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_7d206f446d34() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."organization_id" IS NULL THEN
  SELECT "organization_id"
  INTO NEW."organization_id"
  FROM "snippet_uploads"
  WHERE "snippet_uploads"."id" = NEW."personal_snippet_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_7de792ddbc05(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_7de792ddbc05() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "dast_site_tokens"
  WHERE "dast_site_tokens"."id" = NEW."dast_site_token_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_7e2eed79e46e(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_7e2eed79e46e() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  NEW."assignee_id_convert_to_bigint" := NEW."assignee_id";
  NEW."id_convert_to_bigint" := NEW."id";
  NEW."reporter_id_convert_to_bigint" := NEW."reporter_id";
  NEW."resolved_by_id_convert_to_bigint" := NEW."resolved_by_id";
  NEW."user_id_convert_to_bigint" := NEW."user_id";
  RETURN NEW;
END;
$$;


--
-- Name: trigger_80578cfbdaf9(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_80578cfbdaf9() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "events"
  WHERE "events"."id" = NEW."event_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_81b4c93e7133(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_81b4c93e7133() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "pages_deployments"
  WHERE "pages_deployments"."id" = NEW."pages_deployment_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_81b53b626109(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_81b53b626109() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "packages_package_files"
  WHERE "packages_package_files"."id" = NEW."package_file_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_8204480b3a2e(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_8204480b3a2e() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "incident_management_escalation_policies"
  WHERE "incident_management_escalation_policies"."id" = NEW."policy_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_84d67ad63e93(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_84d67ad63e93() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "wiki_page_meta"
  WHERE "wiki_page_meta"."id" = NEW."wiki_page_meta_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_85d89f0f11db(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_85d89f0f11db() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issues"
  WHERE "issues"."id" = NEW."issue_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_8a11b103857c(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_8a11b103857c() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."group_id" IS NULL THEN
  SELECT "group_id"
  INTO NEW."group_id"
  FROM "packages_debian_group_components"
  WHERE "packages_debian_group_components"."id" = NEW."component_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_8a38ce2327de(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_8a38ce2327de() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."group_id" IS NULL THEN
  SELECT "group_id"
  INTO NEW."group_id"
  FROM "epics"
  WHERE "epics"."id" = NEW."epic_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_8ac78f164b2d(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_8ac78f164b2d() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "project_namespace_id"
  INTO NEW."namespace_id"
  FROM "projects"
  WHERE "projects"."id" = NEW."project_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_8b39d532224c(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_8b39d532224c() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "ci_secure_files"
  WHERE "ci_secure_files"."id" = NEW."ci_secure_file_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_8ba074736a77(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_8ba074736a77() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."snippet_project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."snippet_project_id"
  FROM "snippets"
  WHERE "snippets"."id" = NEW."snippet_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_8cb8ad095bf6(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_8cb8ad095bf6() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "bulk_import_entities"
  WHERE "bulk_import_entities"."id" = NEW."bulk_import_entity_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_8cf1745cf163(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_8cf1745cf163() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "design_management_repositories"
  WHERE "design_management_repositories"."id" = NEW."design_management_repository_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_8d002f38bdef(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_8d002f38bdef() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."group_id" IS NULL THEN
  SELECT "group_id"
  INTO NEW."group_id"
  FROM "packages_debian_group_distributions"
  WHERE "packages_debian_group_distributions"."id" = NEW."distribution_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_8d17725116fe(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_8d17725116fe() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "target_project_id"
  INTO NEW."project_id"
  FROM "merge_requests"
  WHERE "merge_requests"."id" = NEW."merge_request_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_8d661362aa1a(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_8d661362aa1a() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issues"
  WHERE "issues"."id" = NEW."issue_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_8e66b994e8f0(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_8e66b994e8f0() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."group_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."group_id"
  FROM "audit_events_external_audit_event_destinations"
  WHERE "audit_events_external_audit_event_destinations"."id" = NEW."external_audit_event_destination_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_8fbb044c64ad(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_8fbb044c64ad() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "project_namespace_id"
  INTO NEW."namespace_id"
  FROM "projects"
  WHERE "projects"."id" = NEW."project_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_90fa5c6951f1(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_90fa5c6951f1() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "dast_profiles"
  WHERE "dast_profiles"."id" = NEW."dast_profile_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_91e1012b9851(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_91e1012b9851() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "merge_request_context_commits"
  WHERE "merge_request_context_commits"."id" = NEW."merge_request_context_commit_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_9259aae92378(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_9259aae92378() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "packages_packages"
  WHERE "packages_packages"."id" = NEW."package_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_93a5b044f4e8(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_93a5b044f4e8() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."snippet_organization_id" IS NULL THEN
  SELECT "organization_id"
  INTO NEW."snippet_organization_id"
  FROM "snippets"
  WHERE "snippets"."id" = NEW."snippet_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_940b0d0d96a8(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_940b0d0d96a8() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "ai_vectorizable_file_uploads"
  WHERE "ai_vectorizable_file_uploads"."id" = NEW."ai_vectorizable_file_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_94514aeadc50(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_94514aeadc50() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "deployments"
  WHERE "deployments"."id" = NEW."deployment_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_951ac22c24d7(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_951ac22c24d7() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."protected_branch_namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."protected_branch_namespace_id"
  FROM "protected_branches"
  WHERE "protected_branches"."id" = NEW."protected_branch_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_96298f7da5d3(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_96298f7da5d3() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."protected_branch_project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."protected_branch_project_id"
  FROM "protected_branches"
  WHERE "protected_branches"."id" = NEW."protected_branch_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_965022e69ca9(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_965022e69ca9() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "bulk_import_export_upload_uploads"
  WHERE "bulk_import_export_upload_uploads"."id" = NEW."bulk_import_export_upload_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_9699ea03bb37(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_9699ea03bb37() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."group_id" IS NULL THEN
  SELECT "group_id"
  INTO NEW."group_id"
  FROM "epics"
  WHERE "epics"."id" = NEW."source_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_96a76ee9f147(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_96a76ee9f147() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issues"
  WHERE "issues"."id" = NEW."issue_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_979e7f45114f(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_979e7f45114f() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "ml_candidates"
  WHERE "ml_candidates"."id" = NEW."candidate_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_97e9245e767d(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_97e9245e767d() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issues"
  WHERE "issues"."id" = NEW."issue_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_98ad3a4c1d35(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_98ad3a4c1d35() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "reviews"
  WHERE "reviews"."id" = NEW."review_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_99fbbdf73a77(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_99fbbdf73a77() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "project_uploads"
  WHERE "project_uploads"."id" = NEW."project_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_9b944f36fdac(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_9b944f36fdac() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "approval_merge_request_rules"
  WHERE "approval_merge_request_rules"."id" = NEW."approval_merge_request_rule_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_9e137c16de79(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_9e137c16de79() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "vulnerability_occurrences"
  WHERE "vulnerability_occurrences"."id" = NEW."vulnerability_occurrence_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_9e875cabe9c9(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_9e875cabe9c9() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "wiki_page_meta"
  WHERE "wiki_page_meta"."id" = NEW."wiki_page_meta_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_9f3745f8fe32(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_9f3745f8fe32() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "target_project_id"
  INTO NEW."project_id"
  FROM "merge_requests"
  WHERE "merge_requests"."id" = NEW."merge_request_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_9f3de326ea61(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_9f3de326ea61() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "ci_pipeline_schedules"
  WHERE "ci_pipeline_schedules"."id" = NEW."pipeline_schedule_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_9f4b9e63e741(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_9f4b9e63e741() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "achievement_uploads"
  WHERE "achievement_uploads"."id" = NEW."achievement_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_a1bc7c70cbdf(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_a1bc7c70cbdf() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "vulnerabilities"
  WHERE "vulnerabilities"."id" = NEW."vulnerability_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_a22be47501db(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_a22be47501db() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."group_id" IS NULL THEN
  SELECT "group_id"
  INTO NEW."group_id"
  FROM "group_wiki_repositories"
  WHERE "group_wiki_repositories"."group_id" = NEW."group_wiki_repository_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_a253cb3cacdf(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_a253cb3cacdf() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "environments"
  WHERE "environments"."id" = NEW."environment_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_a3bf14aafa32(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_a3bf14aafa32() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."snippet_project_id" IS NULL THEN
  SELECT "snippet_project_id"
  INTO NEW."snippet_project_id"
  FROM "snippet_repositories"
  WHERE "snippet_repositories"."snippet_id" = NEW."snippet_repository_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_a465de38164e(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_a465de38164e() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "p_ci_job_artifacts"
  WHERE "p_ci_job_artifacts"."id" = NEW."job_artifact_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_a4e4fb2451d9(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_a4e4fb2451d9() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."group_id" IS NULL THEN
  SELECT "group_id"
  INTO NEW."group_id"
  FROM "epics"
  WHERE "epics"."id" = NEW."epic_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_a5ad4291f3cc(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_a5ad4291f3cc() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "namespace_uploads"
  WHERE "namespace_uploads"."id" = NEW."group_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_a68471fea292(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_a68471fea292() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."organization_id" IS NULL THEN
  SELECT "organization_id"
  INTO NEW."organization_id"
  FROM "snippets"
  WHERE "snippets"."id" = NEW."model_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_a7e0fb195210(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_a7e0fb195210() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "vulnerability_occurrences"
  WHERE "vulnerability_occurrences"."id" = NEW."vulnerability_occurrence_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_ad05b7ebe49b(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_ad05b7ebe49b() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "deployments"
  WHERE "deployments"."id" = NEW."deployment_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_af3f17817e4d(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_af3f17817e4d() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "protected_tags"
  WHERE "protected_tags"."id" = NEW."protected_tag_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_b046dd50c711(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_b046dd50c711() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "incident_management_oncall_schedules"
  WHERE "incident_management_oncall_schedules"."id" = NEW."oncall_schedule_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_b04dea279493(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_b04dea279493() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."organization_id" IS NULL THEN
  SELECT "organization_id"
  INTO NEW."organization_id"
  FROM "project_topic_uploads"
  WHERE "project_topic_uploads"."id" = NEW."project_topic_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_b0f4298cadff(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_b0f4298cadff() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."protected_branch_project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."protected_branch_project_id"
  FROM "protected_branches"
  WHERE "protected_branches"."id" = NEW."protected_branch_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_b2612138515d(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_b2612138515d() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "project_export_jobs"
  WHERE "project_export_jobs"."id" = NEW."project_export_job_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_b4520c29ea74(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_b4520c29ea74() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "approval_project_rules"
  WHERE "approval_project_rules"."id" = NEW."approval_project_rule_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_b56b0ea1c259(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_b56b0ea1c259() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "design_management_action_uploads"
  WHERE "design_management_action_uploads"."id" = NEW."design_management_action_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_b75e5731e305(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_b75e5731e305() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "dast_profiles"
  WHERE "dast_profiles"."id" = NEW."dast_profile_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_b7abb8fc4cf0(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_b7abb8fc4cf0() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issues"
  WHERE "issues"."id" = NEW."issue_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_b83b7e51e2f5(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_b83b7e51e2f5() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL AND NEW."project_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "security_orchestration_policy_configurations"
  WHERE "security_orchestration_policy_configurations"."id" = NEW."security_orchestration_policy_configuration_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_b8eecea7f351(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_b8eecea7f351() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."group_id" IS NULL THEN
  SELECT "group_id"
  INTO NEW."group_id"
  FROM "dependency_proxy_manifests"
  WHERE "dependency_proxy_manifests"."id" = NEW."dependency_proxy_manifest_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_c17a166692a2(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_c17a166692a2() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."group_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."group_id"
  FROM "audit_events_external_audit_event_destinations"
  WHERE "audit_events_external_audit_event_destinations"."id" = NEW."external_audit_event_destination_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_c24a252f7b04(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_c24a252f7b04() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "design_management_designs_versions"
  WHERE "design_management_designs_versions"."id" = NEW."model_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_c40a5bb7c1c3(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_c40a5bb7c1c3() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "bulk_import_export_uploads"
  WHERE "bulk_import_export_uploads"."id" = NEW."model_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_c4f5bed67b15(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_c4f5bed67b15() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "alert_management_alert_metric_image_uploads"
  WHERE "alert_management_alert_metric_image_uploads"."id" = NEW."alert_management_metric_image_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_c52d215d50a1(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_c52d215d50a1() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issues"
  WHERE "issues"."id" = NEW."issue_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_c59fe6f31e71(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_c59fe6f31e71() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "security_orchestration_policy_configurations"
  WHERE "security_orchestration_policy_configurations"."id" = NEW."security_orchestration_policy_configuration_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_c5eec113ea76(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_c5eec113ea76() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "dast_profiles"
  WHERE "dast_profiles"."id" = NEW."dast_profile_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_c6728503decb(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_c6728503decb() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "design_management_designs"
  WHERE "design_management_designs"."id" = NEW."design_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_c8bb98475baa(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_c8bb98475baa() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "packages_package_files"
  WHERE "packages_package_files"."id" = NEW."package_file_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_c8bc8646bce9(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_c8bc8646bce9() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "vulnerabilities"
  WHERE "vulnerabilities"."id" = NEW."vulnerability_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_c9090feed334(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_c9090feed334() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."group_id" IS NULL THEN
  SELECT "group_id"
  INTO NEW."group_id"
  FROM "boards_epic_boards"
  WHERE "boards_epic_boards"."id" = NEW."epic_board_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_ca93521f3a6d(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_ca93521f3a6d() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."organization_id" IS NULL THEN
  SELECT "organization_id"
  INTO NEW."organization_id"
  FROM "abuse_reports"
  WHERE "abuse_reports"."id" = NEW."abuse_report_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_cac7c0698291(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_cac7c0698291() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "releases"
  WHERE "releases"."id" = NEW."release_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_cb4808fcaffa(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_cb4808fcaffa() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issuable_metric_image_uploads"
  WHERE "issuable_metric_image_uploads"."id" = NEW."issuable_metric_image_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_cbb818bdb3e8(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_cbb818bdb3e8() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "project_import_export_relation_export_upload_uploads"
  WHERE "project_import_export_relation_export_upload_uploads"."id" = NEW."project_import_export_relation_export_upload_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_cca6a43d90dd(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_cca6a43d90dd() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "achievements"
  WHERE "achievements"."id" = NEW."model_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_cd50823537a3(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_cd50823537a3() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issues"
  WHERE "issues"."id" = NEW."issue_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_cdfa6500a121(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_cdfa6500a121() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."snippet_organization_id" IS NULL THEN
  SELECT "organization_id"
  INTO NEW."snippet_organization_id"
  FROM "snippets"
  WHERE "snippets"."id" = NEW."snippet_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_cf646a118cbb(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_cf646a118cbb() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "releases"
  WHERE "releases"."id" = NEW."release_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_cfbec3f07e2b(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_cfbec3f07e2b() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "deployments"
  WHERE "deployments"."id" = NEW."deployment_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_d32ff9d5c63d(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_d32ff9d5c63d() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "group_id"
  INTO NEW."namespace_id"
  FROM "bulk_import_export_uploads"
  WHERE "bulk_import_export_uploads"."id" = NEW."model_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_d4487a75bd44(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_d4487a75bd44() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "terraform_states"
  WHERE "terraform_states"."id" = NEW."terraform_state_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_d5c895007948(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_d5c895007948() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."protected_environment_project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."protected_environment_project_id"
  FROM "protected_environments"
  WHERE "protected_environments"."id" = NEW."protected_environment_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_d8c2de748d8c(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_d8c2de748d8c() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "target_project_id"
  INTO NEW."project_id"
  FROM "merge_requests"
  WHERE "merge_requests"."id" = NEW."merge_request_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_d9468bfbb0b4(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_d9468bfbb0b4() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."snippet_project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."snippet_project_id"
  FROM "snippets"
  WHERE "snippets"."id" = NEW."snippet_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_da5fd3d6d75c(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_da5fd3d6d75c() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "packages_packages"
  WHERE "packages_packages"."id" = NEW."package_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_dadd660afe2c(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_dadd660afe2c() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."group_id" IS NULL THEN
  SELECT "group_id"
  INTO NEW."group_id"
  FROM "packages_debian_group_distributions"
  WHERE "packages_debian_group_distributions"."id" = NEW."distribution_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_dbdd61a66a91(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_dbdd61a66a91() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."agent_project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."agent_project_id"
  FROM "cluster_agents"
  WHERE "cluster_agents"."id" = NEW."agent_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_dbe374a57cbb(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_dbe374a57cbb() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issues"
  WHERE "issues"."id" = NEW."issue_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_dc13168b8025(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_dc13168b8025() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "vulnerability_occurrences"
  WHERE "vulnerability_occurrences"."id" = NEW."vulnerability_occurrence_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_de59b81d3044(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_de59b81d3044() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."group_id" IS NULL THEN
  SELECT "group_id"
  INTO NEW."group_id"
  FROM "bulk_import_exports"
  WHERE "bulk_import_exports"."id" = NEW."export_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_decac6b7c511(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_decac6b7c511() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."snippet_organization_id" IS NULL THEN
  SELECT "snippet_organization_id"
  INTO NEW."snippet_organization_id"
  FROM "snippet_repositories"
  WHERE "snippet_repositories"."snippet_id" = NEW."snippet_repository_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_dfad97659d5f(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_dfad97659d5f() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issues"
  WHERE "issues"."id" = NEW."issue_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_e0864d1cff37(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_e0864d1cff37() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."group_id" IS NULL THEN
  SELECT "group_id"
  INTO NEW."group_id"
  FROM "packages_debian_group_distributions"
  WHERE "packages_debian_group_distributions"."id" = NEW."distribution_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_e1da4a738230(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_e1da4a738230() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "vulnerabilities"
  WHERE "vulnerabilities"."id" = NEW."vulnerability_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_e49ab4d904a0(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_e49ab4d904a0() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "vulnerability_occurrences"
  WHERE "vulnerability_occurrences"."id" = NEW."vulnerability_occurrence_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_e4a6cde57b42(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_e4a6cde57b42() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issues"
  WHERE "issues"."id" = NEW."issue_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_e740510cfd33(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_e740510cfd33() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issuable_metric_images"
  WHERE "issuable_metric_images"."id" = NEW."model_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_e815625b59fa(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_e815625b59fa() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issues"
  WHERE "issues"."id" = NEW."issue_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_ebab34f83f1d(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_ebab34f83f1d() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "packages_packages"
  WHERE "packages_packages"."id" = NEW."package_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_ec1934755627(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_ec1934755627() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "alert_management_alerts"
  WHERE "alert_management_alerts"."id" = NEW."alert_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_ed554313ca66(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_ed554313ca66() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."protected_branch_namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."protected_branch_namespace_id"
  FROM "protected_branches"
  WHERE "protected_branches"."id" = NEW."protected_branch_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_efb9d354f05a(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_efb9d354f05a() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "issues"
  WHERE "issues"."id" = NEW."issue_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_eff80ead42ac(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_eff80ead42ac() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "ci_unit_tests"
  WHERE "ci_unit_tests"."id" = NEW."unit_test_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_f468204dcd5d(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_f468204dcd5d() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "project_relation_export_uploads"
  WHERE "project_relation_export_uploads"."id" = NEW."model_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_f6c61cdddf31(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_f6c61cdddf31() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "ml_models"
  WHERE "ml_models"."id" = NEW."model_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_f6f59d8216b3(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_f6f59d8216b3() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."protected_environment_group_id" IS NULL THEN
  SELECT "group_id"
  INTO NEW."protected_environment_group_id"
  FROM "protected_environments"
  WHERE "protected_environments"."id" = NEW."protected_environment_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_f7464057d53e(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_f7464057d53e() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."organization_id" IS NULL THEN
  SELECT "organization_id"
  INTO NEW."organization_id"
  FROM "users"
  WHERE "users"."id" = NEW."reporter_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_fa69822b05a9(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_fa69822b05a9() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."uploaded_by_user_id" IS NULL THEN
  SELECT "uploaded_by_user_id"
  INTO NEW."uploaded_by_user_id"
  FROM "user_permission_export_upload_uploads"
  WHERE "user_permission_export_upload_uploads"."id" = NEW."user_permission_export_upload_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_fac444e0cae6(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_fac444e0cae6() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."namespace_id" IS NULL THEN
  SELECT "namespace_id"
  INTO NEW."namespace_id"
  FROM "design_management_designs"
  WHERE "design_management_designs"."id" = NEW."design_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_fbd42ed69453(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_fbd42ed69453() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "external_status_checks"
  WHERE "external_status_checks"."id" = NEW."external_status_check_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_fbd8825b3057(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_fbd8825b3057() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."group_id" IS NULL THEN
  SELECT "group_id"
  INTO NEW."group_id"
  FROM "boards_epic_boards"
  WHERE "boards_epic_boards"."id" = NEW."epic_board_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_fcc3ea1f9d4e(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_fcc3ea1f9d4e() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "ai_vectorizable_files"
  WHERE "ai_vectorizable_files"."id" = NEW."model_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_fd1d6f1b9e4f(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_fd1d6f1b9e4f() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."organization_id" IS NULL THEN
  SELECT "organization_id"
  INTO NEW."organization_id"
  FROM "abuse_report_uploads"
  WHERE "abuse_report_uploads"."id" = NEW."abuse_report_upload_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_fd4a1be98713(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_fd4a1be98713() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "container_repositories"
  WHERE "container_repositories"."id" = NEW."container_repository_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_fff8735b6b9a(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_fff8735b6b9a() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW."project_id" IS NULL THEN
  SELECT "project_id"
  INTO NEW."project_id"
  FROM "vulnerability_occurrences"
  WHERE "vulnerability_occurrences"."id" = NEW."finding_id";
END IF;

RETURN NEW;

END
$$;


--
-- Name: trigger_web_hook_logs_daily_assign_sharding_keys(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trigger_web_hook_logs_daily_assign_sharding_keys() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF num_nonnulls(NEW.organization_id, NEW.project_id, NEW.group_id) <> 1 THEN
  SELECT organization_id, project_id, group_id
  INTO NEW.organization_id, NEW.project_id, NEW.group_id
  FROM web_hooks
  WHERE web_hooks.id = NEW.web_hook_id;
END IF;

RETURN NEW;

END
$$;


--
-- Name: unset_has_issues_on_vulnerability_reads(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.unset_has_issues_on_vulnerability_reads() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
  has_issue_links integer;
BEGIN
  IF (SELECT current_setting('vulnerability_management.dont_execute_db_trigger', true) = 'true') THEN
    RETURN NULL;
  END IF;

  PERFORM 1
  FROM
    vulnerability_reads
  WHERE
    vulnerability_id = OLD.vulnerability_id
  FOR UPDATE;

  SELECT 1 INTO has_issue_links FROM vulnerability_issue_links WHERE vulnerability_id = OLD.vulnerability_id LIMIT 1;

  IF (has_issue_links = 1) THEN
    RETURN NULL;
  END IF;

  UPDATE
    vulnerability_reads
  SET
    has_issues = false
  WHERE
    vulnerability_id = OLD.vulnerability_id;

  RETURN NULL;
END
$$;


--
-- Name: unset_has_merge_request_on_vulnerability_reads(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.unset_has_merge_request_on_vulnerability_reads() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
  has_merge_request_links integer;
BEGIN
  IF (SELECT current_setting('vulnerability_management.dont_execute_db_trigger', true) = 'true') THEN
    RETURN NULL;
  END IF;

  PERFORM 1
  FROM
    vulnerability_reads
  WHERE
    vulnerability_id = OLD.vulnerability_id
  FOR UPDATE;

  SELECT 1 INTO has_merge_request_links FROM vulnerability_merge_request_links WHERE vulnerability_id = OLD.vulnerability_id LIMIT 1;

  IF (has_merge_request_links = 1) THEN
    RETURN NULL;
  END IF;

  UPDATE
    vulnerability_reads
  SET
    has_merge_request = false
  WHERE
    vulnerability_id = OLD.vulnerability_id;

  RETURN NULL;
END
$$;


--
-- Name: update_jira_tracker_data_sharding_key(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.update_jira_tracker_data_sharding_key() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
SELECT
  "integrations"."project_id",
  "integrations"."group_id",
  "integrations"."organization_id"
INTO
  NEW."project_id",
  NEW."group_id",
  NEW."organization_id"
FROM "integrations"
WHERE "integrations"."id" = NEW."integration_id";
RETURN NEW;

END
$$;


--
-- Name: update_location_from_vulnerability_occurrences(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.update_location_from_vulnerability_occurrences() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
UPDATE
  vulnerability_reads
SET
  location_image = NEW.location->>'image',
  casted_cluster_agent_id = CAST(NEW.location->'kubernetes_resource'->>'agent_id' AS bigint),
  cluster_agent_id = NEW.location->'kubernetes_resource'->>'agent_id'
WHERE
  vulnerability_id = NEW.vulnerability_id;
RETURN NULL;

END
$$;


--
-- Name: update_namespace_details_from_projects(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.update_namespace_details_from_projects() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
INSERT INTO
  namespace_details (
    description,
    description_html,
    cached_markdown_version,
    updated_at,
    created_at,
    namespace_id
  )
VALUES
  (
    NEW.description,
    NEW.description_html,
    NEW.cached_markdown_version,
    NEW.updated_at,
    NEW.updated_at,
    NEW.project_namespace_id
  ) ON CONFLICT (namespace_id) DO
UPDATE
SET
  description = NEW.description,
  description_html = NEW.description_html,
  cached_markdown_version = NEW.cached_markdown_version,
  updated_at = NEW.updated_at
WHERE
  namespace_details.namespace_id = NEW.project_namespace_id;RETURN NULL;

END
$$;


--
-- Name: update_vulnerability_reads_from_vulnerability(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.update_vulnerability_reads_from_vulnerability() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
UPDATE
  vulnerability_reads
SET
  severity = NEW.severity,
  state = NEW.state,
  resolved_on_default_branch = NEW.resolved_on_default_branch,
  auto_resolved = NEW.auto_resolved
WHERE vulnerability_id = NEW.id;
RETURN NULL;

END
$$;


--
-- Name: validate_work_item_type_id_is_valid(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.validate_work_item_type_id_is_valid() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF NEW.work_item_type_id >= 1001 THEN
  IF NOT check_work_item_custom_type_exists(NEW.work_item_type_id) THEN
    RAISE EXCEPTION
      'Specified custom work item type does not exist: %',
      NEW.work_item_type_id;
  END IF;
ELSIF NEW.work_item_type_id > 9 THEN
  RAISE EXCEPTION
    'Specified system defined work item type does not exist: %',
    NEW.work_item_type_id;
END IF;

RETURN NEW;

END
$$;


--
-- Name: work_item_custom_types_integrity_children_check(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.work_item_custom_types_integrity_children_check() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
IF exists_issues_for_work_item_custom_type(OLD.id) THEN
  RAISE EXCEPTION
    'Cannot delete work_item_custom_type %, referenced in issues',
    OLD.id;
END IF;

RETURN OLD;

END
$$;


--
-- Name: abuse_reports; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.abuse_reports (
    id bigint NOT NULL,
    reporter_id bigint,
    user_id bigint,
    message text,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    message_html text,
    cached_markdown_version integer,
    category smallint DEFAULT 1 NOT NULL,
    reported_from_url text DEFAULT ''::text NOT NULL,
    links_to_spam text[] DEFAULT '{}'::text[] NOT NULL,
    status smallint DEFAULT 1 NOT NULL,
    resolved_at timestamp with time zone,
    screenshot text,
    resolved_by_id bigint,
    assignee_id bigint,
    mitigation_steps text,
    evidence jsonb,
    assignee_id_convert_to_bigint bigint,
    id_convert_to_bigint bigint DEFAULT 0 NOT NULL,
    reporter_id_convert_to_bigint bigint,
    resolved_by_id_convert_to_bigint bigint,
    user_id_convert_to_bigint bigint,
    organization_id bigint,
    CONSTRAINT abuse_reports_links_to_spam_length_check CHECK ((cardinality(links_to_spam) <= 20)),
    CONSTRAINT check_1e642c5f94 CHECK ((organization_id IS NOT NULL)),
    CONSTRAINT check_4b0a5120e0 CHECK ((char_length(screenshot) <= 255)),
    CONSTRAINT check_ab1260fa6c CHECK ((char_length(reported_from_url) <= 512)),
    CONSTRAINT check_f3c0947a2d CHECK ((char_length(mitigation_steps) <= 1000)),
    CONSTRAINT check_fc643d4880 CHECK ((reporter_id IS NOT NULL))
);


--
-- Name: abuse_reports_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.abuse_reports_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: abuse_reports_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.abuse_reports_id_seq OWNED BY public.abuse_reports.id;


--
-- Name: audit_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.audit_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: award_emoji; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.award_emoji (
    id bigint NOT NULL,
    name character varying,
    user_id bigint,
    awardable_type character varying,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    awardable_id bigint,
    namespace_id bigint,
    organization_id bigint,
    CONSTRAINT check_8ef14b7067 CHECK ((num_nonnulls(namespace_id, organization_id) = 1))
);


--
-- Name: award_emoji_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.award_emoji_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: award_emoji_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.award_emoji_id_seq OWNED BY public.award_emoji.id;


--
-- Name: catalog_resources; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.catalog_resources (
    id bigint NOT NULL,
    project_id bigint NOT NULL,
    created_at timestamp with time zone NOT NULL,
    state smallint DEFAULT 0 NOT NULL,
    latest_released_at timestamp with time zone,
    name character varying,
    description text,
    visibility_level integer DEFAULT 0 NOT NULL,
    search_vector tsvector GENERATED ALWAYS AS ((setweight(to_tsvector('english'::regconfig, (COALESCE(name, ''::character varying))::text), 'A'::"char") || setweight(to_tsvector('english'::regconfig, COALESCE(description, ''::text)), 'B'::"char"))) STORED,
    verification_level smallint DEFAULT 0,
    last_30_day_usage_count integer DEFAULT 0 NOT NULL,
    last_30_day_usage_count_updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: catalog_resources_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.catalog_resources_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: catalog_resources_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.catalog_resources_id_seq OWNED BY public.catalog_resources.id;


--
-- Name: compliance_management_frameworks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.compliance_management_frameworks (
    id bigint NOT NULL,
    name text NOT NULL,
    description text NOT NULL,
    color text NOT NULL,
    namespace_id bigint NOT NULL,
    pipeline_configuration_full_path text,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    source_id bigint,
    template_id text,
    template_version integer,
    CONSTRAINT check_08cd34b2c2 CHECK ((char_length(color) <= 10)),
    CONSTRAINT check_1617e0b87e CHECK ((char_length(description) <= 255)),
    CONSTRAINT check_ab00bc2193 CHECK ((char_length(name) <= 255)),
    CONSTRAINT check_e7a9972435 CHECK ((char_length(pipeline_configuration_full_path) <= 255))
);


--
-- Name: compliance_management_frameworks_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.compliance_management_frameworks_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: compliance_management_frameworks_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.compliance_management_frameworks_id_seq OWNED BY public.compliance_management_frameworks.id;


--
-- Name: emails; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.emails (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    email character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    confirmation_token character varying,
    confirmed_at timestamp without time zone,
    confirmation_sent_at timestamp without time zone,
    detumbled_email text,
    organization_id bigint,
    CONSTRAINT check_319f6999dc CHECK ((char_length(detumbled_email) <= 255))
);


--
-- Name: emails_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.emails_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: emails_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.emails_id_seq OWNED BY public.emails.id;


--
-- Name: epics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.epics (
    id bigint NOT NULL,
    group_id bigint NOT NULL,
    author_id bigint NOT NULL,
    assignee_id bigint,
    iid integer NOT NULL,
    cached_markdown_version integer,
    updated_by_id bigint,
    last_edited_by_id bigint,
    lock_version integer DEFAULT 0,
    start_date date,
    end_date date,
    last_edited_at timestamp without time zone,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    title character varying NOT NULL,
    title_html character varying NOT NULL,
    description text,
    description_html text,
    start_date_sourcing_milestone_id bigint,
    due_date_sourcing_milestone_id bigint,
    start_date_fixed date,
    due_date_fixed date,
    start_date_is_fixed boolean,
    due_date_is_fixed boolean,
    closed_by_id bigint,
    closed_at timestamp without time zone,
    parent_id bigint,
    relative_position integer,
    state_id smallint DEFAULT 1 NOT NULL,
    start_date_sourcing_epic_id bigint,
    due_date_sourcing_epic_id bigint,
    confidential boolean DEFAULT false NOT NULL,
    external_key character varying(255),
    color text DEFAULT '#1068bf'::text,
    total_opened_issue_weight integer DEFAULT 0 NOT NULL,
    total_closed_issue_weight integer DEFAULT 0 NOT NULL,
    total_opened_issue_count integer DEFAULT 0 NOT NULL,
    total_closed_issue_count integer DEFAULT 0 NOT NULL,
    issue_id bigint,
    imported smallint DEFAULT 0 NOT NULL,
    imported_from smallint DEFAULT 0 NOT NULL,
    work_item_parent_link_id bigint,
    CONSTRAINT check_450724d1bb CHECK ((issue_id IS NOT NULL)),
    CONSTRAINT check_ca608c40b3 CHECK ((char_length(color) <= 7)),
    CONSTRAINT check_fcfb4a93ff CHECK ((lock_version IS NOT NULL))
);


--
-- Name: epics_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.epics_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: epics_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.epics_id_seq OWNED BY public.epics.id;


--
-- Name: events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.events (
    project_id bigint,
    author_id bigint NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    action smallint NOT NULL,
    target_type character varying,
    group_id bigint,
    fingerprint bytea,
    id bigint NOT NULL,
    target_id bigint,
    imported_from smallint DEFAULT 0 NOT NULL,
    personal_namespace_id bigint,
    CONSTRAINT check_97e06e05ad CHECK ((octet_length(fingerprint) <= 128)),
    CONSTRAINT check_events_sharding_key_is_not_null CHECK (((group_id IS NOT NULL) OR (project_id IS NOT NULL) OR (personal_namespace_id IS NOT NULL)))
);


--
-- Name: events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: events_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.events_id_seq OWNED BY public.events.id;


--
-- Name: identities; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.identities (
    id bigint NOT NULL,
    extern_uid character varying,
    provider character varying,
    user_id bigint,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    secondary_extern_uid character varying,
    saml_provider_id bigint,
    trusted_extern_uid boolean DEFAULT true,
    CONSTRAINT check_e6693ca8db CHECK ((user_id IS NOT NULL))
);


--
-- Name: identities_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.identities_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: identities_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.identities_id_seq OWNED BY public.identities.id;


--
-- Name: issue_assignees; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.issue_assignees (
    user_id bigint NOT NULL,
    issue_id bigint NOT NULL,
    namespace_id bigint,
    CONSTRAINT check_d88fe18cfa CHECK ((namespace_id IS NOT NULL))
);


--
-- Name: issues; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.issues (
    id bigint NOT NULL,
    title character varying,
    author_id bigint,
    project_id bigint,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    description text,
    milestone_id bigint,
    iid integer,
    updated_by_id bigint,
    weight integer,
    confidential boolean DEFAULT false NOT NULL,
    due_date date,
    moved_to_id bigint,
    lock_version integer DEFAULT 0,
    title_html text,
    description_html text,
    time_estimate integer DEFAULT 0,
    relative_position integer,
    service_desk_reply_to character varying,
    cached_markdown_version integer,
    last_edited_at timestamp without time zone,
    last_edited_by_id bigint,
    discussion_locked boolean,
    closed_at timestamp with time zone,
    closed_by_id bigint,
    state_id smallint DEFAULT 1 NOT NULL,
    duplicated_to_id bigint,
    promoted_to_epic_id bigint,
    health_status smallint,
    sprint_id bigint,
    blocking_issues_count integer DEFAULT 0 NOT NULL,
    upvotes_count integer DEFAULT 0 NOT NULL,
    work_item_type_id bigint,
    namespace_id bigint,
    start_date date,
    imported_from smallint DEFAULT 0 NOT NULL,
    namespace_traversal_ids bigint[] DEFAULT '{}'::bigint[],
    CONSTRAINT check_2addf801cd CHECK ((work_item_type_id IS NOT NULL)),
    CONSTRAINT check_c33362cd43 CHECK ((namespace_id IS NOT NULL)),
    CONSTRAINT check_fba63f706d CHECK ((lock_version IS NOT NULL))
);


--
-- Name: issues_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.issues_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: issues_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.issues_id_seq OWNED BY public.issues.id;


--
-- Name: iterations_cadences; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.iterations_cadences (
    id bigint NOT NULL,
    group_id bigint NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    start_date date,
    duration_in_weeks integer,
    iterations_in_advance integer,
    active boolean DEFAULT true NOT NULL,
    automatic boolean DEFAULT true NOT NULL,
    title text NOT NULL,
    roll_over boolean DEFAULT false NOT NULL,
    description text,
    next_run_date date,
    CONSTRAINT check_5c5d2b44bd CHECK ((char_length(description) <= 5000)),
    CONSTRAINT check_fedff82d3b CHECK ((char_length(title) <= 255))
);


--
-- Name: iterations_cadences_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.iterations_cadences_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: iterations_cadences_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.iterations_cadences_id_seq OWNED BY public.iterations_cadences.id;


--
-- Name: loose_foreign_keys_deleted_records; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.loose_foreign_keys_deleted_records (
    id bigint NOT NULL,
    partition bigint DEFAULT 1 NOT NULL,
    primary_key_value bigint NOT NULL,
    status smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    fully_qualified_table_name text NOT NULL,
    consume_after timestamp with time zone DEFAULT now(),
    cleanup_attempts smallint DEFAULT 0,
    CONSTRAINT check_1a541f3235 CHECK ((char_length(fully_qualified_table_name) <= 150))
)
PARTITION BY LIST (partition);


--
-- Name: loose_foreign_keys_deleted_records_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.loose_foreign_keys_deleted_records_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: loose_foreign_keys_deleted_records_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.loose_foreign_keys_deleted_records_id_seq OWNED BY public.loose_foreign_keys_deleted_records.id;


--
-- Name: member_roles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.member_roles (
    id bigint NOT NULL,
    namespace_id bigint,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    base_access_level integer,
    name text DEFAULT 'Custom'::text NOT NULL,
    description text,
    occupies_seat boolean DEFAULT false NOT NULL,
    permissions jsonb DEFAULT '{}'::jsonb NOT NULL,
    organization_id bigint,
    CONSTRAINT check_4364846f58 CHECK ((char_length(description) <= 255)),
    CONSTRAINT check_9907916995 CHECK ((char_length(name) <= 255)),
    CONSTRAINT check_ae96d7c575 CHECK ((num_nonnulls(namespace_id, organization_id) = 1))
);


--
-- Name: member_roles_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.member_roles_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: member_roles_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.member_roles_id_seq OWNED BY public.member_roles.id;


--
-- Name: members; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.members (
    id bigint NOT NULL,
    access_level integer NOT NULL,
    source_id bigint NOT NULL,
    source_type character varying NOT NULL,
    user_id bigint,
    notification_level integer NOT NULL,
    type character varying,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    created_by_id bigint,
    invite_email character varying,
    invite_token character varying,
    invite_accepted_at timestamp without time zone,
    requested_at timestamp without time zone,
    expires_at date,
    ldap boolean DEFAULT false NOT NULL,
    override boolean DEFAULT false NOT NULL,
    state smallint DEFAULT 0,
    invite_email_success boolean DEFAULT true NOT NULL,
    member_namespace_id bigint,
    member_role_id bigint,
    expiry_notified_at timestamp with time zone,
    request_accepted_at timestamp with time zone,
    CONSTRAINT check_508774aac0 CHECK ((member_namespace_id IS NOT NULL))
);


--
-- Name: members_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.members_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: members_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.members_id_seq OWNED BY public.members.id;


--
-- Name: merge_request_diffs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.merge_request_diffs (
    id bigint NOT NULL,
    state character varying,
    merge_request_id bigint NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    base_commit_sha character varying,
    real_size character varying,
    head_commit_sha character varying,
    start_commit_sha character varying,
    commits_count integer,
    external_diff character varying,
    external_diff_store integer DEFAULT 1,
    stored_externally boolean,
    files_count smallint,
    sorted boolean DEFAULT false NOT NULL,
    diff_type smallint DEFAULT 1 NOT NULL,
    patch_id_sha bytea,
    project_id bigint,
    base_commit_sha_bytea bytea,
    start_commit_sha_bytea bytea,
    head_commit_sha_bytea bytea,
    CONSTRAINT check_11c5f029ad CHECK ((project_id IS NOT NULL)),
    CONSTRAINT check_93ee616ac9 CHECK ((external_diff_store IS NOT NULL))
);


--
-- Name: merge_request_diffs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.merge_request_diffs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: merge_request_diffs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.merge_request_diffs_id_seq OWNED BY public.merge_request_diffs.id;


--
-- Name: merge_requests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.merge_requests (
    id bigint NOT NULL,
    target_branch character varying NOT NULL,
    source_branch character varying NOT NULL,
    source_project_id bigint,
    author_id bigint,
    assignee_id bigint,
    title character varying,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    milestone_id bigint,
    merge_status character varying DEFAULT 'unchecked'::character varying NOT NULL,
    target_project_id bigint NOT NULL,
    iid integer,
    description text,
    updated_by_id bigint,
    merge_error text,
    merge_params text,
    merge_when_pipeline_succeeds boolean DEFAULT false NOT NULL,
    merge_user_id bigint,
    merge_commit_sha character varying,
    approvals_before_merge integer,
    rebase_commit_sha character varying,
    in_progress_merge_commit_sha character varying,
    lock_version integer DEFAULT 0,
    title_html text,
    description_html text,
    time_estimate integer DEFAULT 0,
    squash boolean DEFAULT false NOT NULL,
    cached_markdown_version integer,
    last_edited_at timestamp without time zone,
    last_edited_by_id bigint,
    merge_jid character varying,
    discussion_locked boolean,
    latest_merge_request_diff_id bigint,
    allow_maintainer_to_push boolean DEFAULT true,
    state_id smallint DEFAULT 1 NOT NULL,
    rebase_jid character varying,
    squash_commit_sha bytea,
    merge_ref_sha bytea,
    draft boolean DEFAULT false NOT NULL,
    prepared_at timestamp with time zone,
    merged_commit_sha bytea,
    override_requested_changes boolean DEFAULT false NOT NULL,
    head_pipeline_id bigint,
    imported_from smallint DEFAULT 0 NOT NULL,
    retargeted boolean DEFAULT false NOT NULL,
    CONSTRAINT check_970d272570 CHECK ((lock_version IS NOT NULL))
);


--
-- Name: merge_requests_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.merge_requests_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: merge_requests_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.merge_requests_id_seq OWNED BY public.merge_requests.id;


--
-- Name: milestones; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.milestones (
    id bigint NOT NULL,
    title character varying NOT NULL,
    project_id bigint,
    description text,
    due_date date,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    state character varying,
    iid integer,
    title_html text,
    description_html text,
    start_date date,
    cached_markdown_version integer,
    group_id bigint,
    lock_version integer DEFAULT 0 NOT NULL,
    CONSTRAINT check_08e9c27987 CHECK ((num_nonnulls(group_id, project_id) = 1))
);


--
-- Name: milestones_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.milestones_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: milestones_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.milestones_id_seq OWNED BY public.milestones.id;


--
-- Name: namespace_details; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.namespace_details (
    namespace_id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    cached_markdown_version integer,
    description text,
    description_html text,
    creator_id bigint,
    state_metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    deletion_scheduled_at timestamp with time zone,
    CONSTRAINT check_namespace_details_state_metadata_is_hash CHECK ((jsonb_typeof(state_metadata) = 'object'::text))
);


--
-- Name: namespace_settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.namespace_settings (
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    namespace_id bigint NOT NULL,
    prevent_forking_outside_group boolean DEFAULT false NOT NULL,
    allow_mfa_for_subgroups boolean DEFAULT true NOT NULL,
    default_branch_name text,
    repository_read_only boolean DEFAULT false NOT NULL,
    resource_access_token_creation_allowed boolean DEFAULT true NOT NULL,
    prevent_sharing_groups_outside_hierarchy boolean DEFAULT false NOT NULL,
    new_user_signups_cap integer,
    setup_for_company boolean,
    jobs_to_be_done smallint,
    runner_token_expiration_interval integer,
    subgroup_runner_token_expiration_interval integer,
    project_runner_token_expiration_interval integer,
    show_diff_preview_in_email boolean DEFAULT true NOT NULL,
    enabled_git_access_protocol smallint DEFAULT 0 NOT NULL,
    unique_project_download_limit smallint DEFAULT 0 NOT NULL,
    unique_project_download_limit_interval_in_seconds integer DEFAULT 0 NOT NULL,
    unique_project_download_limit_allowlist text[] DEFAULT '{}'::text[] NOT NULL,
    auto_ban_user_on_excessive_projects_download boolean DEFAULT false NOT NULL,
    only_allow_merge_if_pipeline_succeeds boolean DEFAULT false NOT NULL,
    allow_merge_on_skipped_pipeline boolean DEFAULT false NOT NULL,
    only_allow_merge_if_all_discussions_are_resolved boolean DEFAULT false NOT NULL,
    default_compliance_framework_id bigint,
    runner_registration_enabled boolean DEFAULT true,
    allow_runner_registration_token boolean DEFAULT true NOT NULL,
    unique_project_download_limit_alertlist integer[] DEFAULT '{}'::integer[] NOT NULL,
    emails_enabled boolean DEFAULT true NOT NULL,
    experiment_features_enabled boolean DEFAULT false NOT NULL,
    default_branch_protection_defaults jsonb DEFAULT '{}'::jsonb NOT NULL,
    service_access_tokens_expiration_enforced boolean DEFAULT true NOT NULL,
    product_analytics_enabled boolean DEFAULT false NOT NULL,
    allow_merge_without_pipeline boolean DEFAULT false NOT NULL,
    enforce_ssh_certificates boolean DEFAULT false NOT NULL,
    math_rendering_limits_enabled boolean,
    lock_math_rendering_limits_enabled boolean DEFAULT false NOT NULL,
    duo_features_enabled boolean,
    lock_duo_features_enabled boolean DEFAULT false NOT NULL,
    disable_personal_access_tokens boolean DEFAULT false NOT NULL,
    early_access_program_participant boolean DEFAULT false NOT NULL,
    remove_dormant_members boolean DEFAULT false NOT NULL,
    remove_dormant_members_period integer DEFAULT 90 NOT NULL,
    early_access_program_joined_by_id bigint,
    seat_control smallint DEFAULT 0 NOT NULL,
    last_dormant_member_review_at timestamp with time zone,
    enterprise_users_extensions_marketplace_opt_in_status smallint DEFAULT 0 NOT NULL,
    spp_repository_pipeline_access boolean,
    lock_spp_repository_pipeline_access boolean DEFAULT false NOT NULL,
    archived boolean DEFAULT false NOT NULL,
    token_expiry_notify_inherited boolean DEFAULT true NOT NULL,
    resource_access_token_notify_inherited boolean,
    lock_resource_access_token_notify_inherited boolean DEFAULT false NOT NULL,
    pipeline_variables_default_role smallint DEFAULT 2 NOT NULL,
    force_pages_access_control boolean DEFAULT false NOT NULL,
    extended_grat_expiry_webhooks_execute boolean DEFAULT false NOT NULL,
    jwt_ci_cd_job_token_enabled boolean DEFAULT false NOT NULL,
    jwt_ci_cd_job_token_opted_out boolean DEFAULT false NOT NULL,
    require_dpop_for_manage_api_endpoints boolean DEFAULT false NOT NULL,
    job_token_policies_enabled boolean DEFAULT false NOT NULL,
    security_policies jsonb DEFAULT '{}'::jsonb NOT NULL,
    duo_nano_features_enabled boolean,
    model_prompt_cache_enabled boolean,
    lock_model_prompt_cache_enabled boolean DEFAULT false NOT NULL,
    disable_invite_members boolean DEFAULT false NOT NULL,
    web_based_commit_signing_enabled boolean,
    lock_web_based_commit_signing_enabled boolean DEFAULT false NOT NULL,
    allow_enterprise_bypass_placeholder_confirmation boolean DEFAULT false NOT NULL,
    enterprise_bypass_expires_at timestamp with time zone,
    hide_email_on_profile boolean DEFAULT false NOT NULL,
    allow_personal_snippets boolean DEFAULT true NOT NULL,
    auto_duo_code_review_enabled boolean,
    lock_auto_duo_code_review_enabled boolean DEFAULT false NOT NULL,
    step_up_auth_required_oauth_provider text,
    duo_remote_flows_enabled boolean,
    lock_duo_remote_flows_enabled boolean DEFAULT false NOT NULL,
    disable_ssh_keys boolean DEFAULT false NOT NULL,
    duo_foundational_flows_enabled boolean,
    lock_duo_foundational_flows_enabled boolean DEFAULT false NOT NULL,
    usage_billing jsonb DEFAULT '{}'::jsonb NOT NULL,
    built_in_project_templates_enabled boolean,
    lock_built_in_project_templates_enabled boolean DEFAULT false NOT NULL,
    tool_approval_for_session_enabled boolean,
    lock_tool_approval_for_session_enabled boolean DEFAULT false NOT NULL,
    duo_custom_flows_enabled boolean,
    lock_duo_custom_flows_enabled boolean DEFAULT false NOT NULL,
    duo_custom_agents_enabled boolean,
    lock_duo_custom_agents_enabled boolean DEFAULT false NOT NULL,
    mcp_server_enabled boolean,
    duo_external_agents_enabled boolean,
    lock_duo_external_agents_enabled boolean DEFAULT false NOT NULL,
    enable_duo_code_review_by_default smallint DEFAULT 0 NOT NULL,
    personal_access_token_settings jsonb DEFAULT '{}'::jsonb NOT NULL,
    require_sha_for_merge boolean,
    lock_require_sha_for_merge boolean DEFAULT false NOT NULL,
    admin_locked_duo_features_enabled boolean DEFAULT false NOT NULL,
    ai_audit_events_storage_enabled boolean,
    lock_ai_audit_events_storage_enabled boolean DEFAULT false NOT NULL,
    dependency_firewall_enabled boolean DEFAULT false NOT NULL,
    policy_store_experiment_enabled boolean DEFAULT false NOT NULL,
    enterprise_user_settings jsonb DEFAULT '{}'::jsonb NOT NULL,
    seat_assignment_model_enabled boolean DEFAULT false NOT NULL,
    ai_custom_instructions text,
    CONSTRAINT check_0ba93c78c7 CHECK ((char_length(default_branch_name) <= 255)),
    CONSTRAINT check_d9644d516f CHECK ((char_length(step_up_auth_required_oauth_provider) <= 255)),
    CONSTRAINT check_namespace_settings_enterprise_user_settings_is_hash CHECK ((jsonb_typeof(enterprise_user_settings) = 'object'::text)),
    CONSTRAINT check_namespace_settings_pat_settings_is_hash CHECK ((jsonb_typeof(personal_access_token_settings) = 'object'::text)),
    CONSTRAINT check_namespace_settings_security_policies_is_hash CHECK ((jsonb_typeof(security_policies) = 'object'::text)),
    CONSTRAINT check_namespace_settings_usage_billing_is_hash CHECK ((jsonb_typeof(usage_billing) = 'object'::text)),
    CONSTRAINT namespace_settings_unique_project_download_limit_alertlist_size CHECK ((cardinality(unique_project_download_limit_alertlist) <= 100)),
    CONSTRAINT namespace_settings_unique_project_download_limit_allowlist_size CHECK ((cardinality(unique_project_download_limit_allowlist) <= 100))
);


--
-- Name: namespaces_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.namespaces_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: namespaces_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.namespaces_id_seq OWNED BY public.namespaces.id;


--
-- Name: namespaces_sync_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.namespaces_sync_events (
    id bigint NOT NULL,
    namespace_id bigint NOT NULL
);


--
-- Name: namespaces_sync_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.namespaces_sync_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: namespaces_sync_events_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.namespaces_sync_events_id_seq OWNED BY public.namespaces_sync_events.id;


--
-- Name: notes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.notes (
    note text,
    noteable_type character varying,
    author_id bigint,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    project_id bigint,
    line_code character varying,
    commit_id character varying,
    noteable_id bigint,
    system boolean DEFAULT false NOT NULL,
    st_diff text,
    updated_by_id bigint,
    type character varying,
    "position" text,
    original_position text,
    resolved_at timestamp without time zone,
    resolved_by_id bigint,
    discussion_id character varying,
    note_html text,
    cached_markdown_version integer,
    change_position text,
    resolved_by_push boolean,
    review_id bigint,
    confidential boolean,
    last_edited_at timestamp with time zone,
    internal boolean DEFAULT false NOT NULL,
    id bigint NOT NULL,
    namespace_id bigint,
    imported_from smallint DEFAULT 0 NOT NULL,
    organization_id bigint,
    CONSTRAINT check_1244cbd7d0 CHECK ((noteable_type IS NOT NULL)),
    CONSTRAINT check_82f260979e CHECK ((num_nonnulls(namespace_id, organization_id, project_id) >= 1))
);


--
-- Name: notes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.notes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: notes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.notes_id_seq OWNED BY public.notes.id;


--
-- Name: organizations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.organizations (
    id bigint NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    name text DEFAULT ''::text NOT NULL,
    path text NOT NULL,
    visibility_level smallint DEFAULT 0 NOT NULL,
    state smallint DEFAULT 0 NOT NULL,
    uuid uuid NOT NULL,
    CONSTRAINT check_0b4296b5ea CHECK ((char_length(path) <= 255)),
    CONSTRAINT check_d130d769e0 CHECK ((char_length(name) <= 255))
);


--
-- Name: organizations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.organizations_id_seq
    START WITH 1000
    INCREMENT BY 1
    MINVALUE 1000
    NO MAXVALUE
    CACHE 1;


--
-- Name: organizations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.organizations_id_seq OWNED BY public.organizations.id;


--
-- Name: p_catalog_resource_sync_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.p_catalog_resource_sync_events (
    id bigint NOT NULL,
    catalog_resource_id bigint NOT NULL,
    project_id bigint NOT NULL,
    partition_id bigint DEFAULT 1 NOT NULL,
    status smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
)
PARTITION BY LIST (partition_id);


--
-- Name: p_catalog_resource_sync_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.p_catalog_resource_sync_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: p_catalog_resource_sync_events_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.p_catalog_resource_sync_events_id_seq OWNED BY public.p_catalog_resource_sync_events.id;


--
-- Name: personal_access_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.personal_access_tokens (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    name character varying NOT NULL,
    revoked boolean DEFAULT false,
    expires_at date,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    scopes character varying DEFAULT '--- []
'::character varying NOT NULL,
    impersonation boolean DEFAULT false NOT NULL,
    token_digest character varying,
    expire_notification_delivered boolean DEFAULT false NOT NULL,
    last_used_at timestamp with time zone,
    after_expiry_notification_delivered boolean DEFAULT false NOT NULL,
    previous_personal_access_token_id bigint,
    organization_id bigint NOT NULL,
    seven_days_notification_sent_at timestamp with time zone,
    thirty_days_notification_sent_at timestamp with time zone,
    sixty_days_notification_sent_at timestamp with time zone,
    description text,
    group_id bigint,
    user_type smallint,
    granular boolean DEFAULT false NOT NULL,
    sudo boolean DEFAULT false NOT NULL,
    CONSTRAINT check_6d2ddc9355 CHECK ((char_length(description) <= 255))
);


--
-- Name: personal_access_tokens_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.personal_access_tokens_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: personal_access_tokens_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.personal_access_tokens_id_seq OWNED BY public.personal_access_tokens.id;


--
-- Name: postgres_autovacuum_activity; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.postgres_autovacuum_activity AS
 WITH processes AS (
         SELECT postgres_pg_stat_activity_autovacuum.query,
            postgres_pg_stat_activity_autovacuum.query_start,
            regexp_matches(postgres_pg_stat_activity_autovacuum.query, '^autovacuum: VACUUM (\w+)\.(\w+)'::text) AS matches,
                CASE
                    WHEN (postgres_pg_stat_activity_autovacuum.query ~~* '%wraparound)'::text) THEN true
                    ELSE false
                END AS wraparound_prevention
           FROM public.postgres_pg_stat_activity_autovacuum() postgres_pg_stat_activity_autovacuum(query, query_start)
          WHERE (postgres_pg_stat_activity_autovacuum.query ~* '^autovacuum: VACUUM \w+\.\w+'::text)
        )
 SELECT ((matches[1] || '.'::text) || matches[2]) AS table_identifier,
    matches[1] AS schema,
    matches[2] AS "table",
    query_start AS vacuum_start,
    wraparound_prevention
   FROM processes;


--
-- Name: VIEW postgres_autovacuum_activity; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON VIEW public.postgres_autovacuum_activity IS 'Contains information about PostgreSQL backends currently performing autovacuum operations on the tables indicated here.';


--
-- Name: postgres_constraints; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.postgres_constraints AS
 SELECT pg_constraint.oid,
    pg_constraint.conname AS name,
    pg_constraint.contype AS constraint_type,
    pg_constraint.convalidated AS constraint_valid,
    ( SELECT array_agg(pg_attribute.attname ORDER BY attnums.ordering) AS array_agg
           FROM (unnest(pg_constraint.conkey) WITH ORDINALITY attnums(attnum, ordering)
             JOIN pg_attribute ON (((pg_attribute.attnum = attnums.attnum) AND (pg_attribute.attrelid = pg_class.oid))))) AS column_names,
    (((pg_namespace.nspname)::text || '.'::text) || (pg_class.relname)::text) AS table_identifier,
    NULLIF(pg_constraint.conparentid, (0)::oid) AS parent_constraint_oid,
    pg_get_constraintdef(pg_constraint.oid) AS definition
   FROM ((pg_constraint
     JOIN pg_class ON ((pg_constraint.conrelid = pg_class.oid)))
     JOIN pg_namespace ON ((pg_class.relnamespace = pg_namespace.oid)))
  WHERE (pg_constraint.contype <> 'n'::"char");


--
-- Name: postgres_foreign_keys; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.postgres_foreign_keys AS
 SELECT pg_constraint.oid,
    pg_constraint.conname AS name,
    (((constrained_namespace.nspname)::text || '.'::text) || (constrained_table.relname)::text) AS constrained_table_identifier,
    (((referenced_namespace.nspname)::text || '.'::text) || (referenced_table.relname)::text) AS referenced_table_identifier,
    (constrained_table.relname)::text AS constrained_table_name,
    (referenced_table.relname)::text AS referenced_table_name,
    constrained_cols.constrained_columns,
    referenced_cols.referenced_columns,
    pg_constraint.confdeltype AS on_delete_action,
    pg_constraint.confupdtype AS on_update_action,
    (pg_constraint.coninhcount > 0) AS is_inherited,
    pg_constraint.convalidated AS is_valid,
    partitioned_parent_oids.parent_oid
   FROM (((((((pg_constraint
     JOIN pg_class constrained_table ON ((constrained_table.oid = pg_constraint.conrelid)))
     JOIN pg_class referenced_table ON ((referenced_table.oid = pg_constraint.confrelid)))
     JOIN pg_namespace constrained_namespace ON ((constrained_table.relnamespace = constrained_namespace.oid)))
     JOIN pg_namespace referenced_namespace ON ((referenced_table.relnamespace = referenced_namespace.oid)))
     CROSS JOIN LATERAL ( SELECT array_agg(pg_attribute.attname ORDER BY conkey.idx) AS array_agg
           FROM (unnest(pg_constraint.conkey) WITH ORDINALITY conkey(attnum, idx)
             JOIN pg_attribute ON (((pg_attribute.attnum = conkey.attnum) AND (pg_attribute.attrelid = constrained_table.oid))))) constrained_cols(constrained_columns))
     CROSS JOIN LATERAL ( SELECT array_agg(pg_attribute.attname ORDER BY confkey.idx) AS array_agg
           FROM (unnest(pg_constraint.confkey) WITH ORDINALITY confkey(attnum, idx)
             JOIN pg_attribute ON (((pg_attribute.attnum = confkey.attnum) AND (pg_attribute.attrelid = referenced_table.oid))))) referenced_cols(referenced_columns))
     LEFT JOIN LATERAL ( SELECT pg_depend.refobjid AS parent_oid
           FROM pg_depend
          WHERE ((pg_depend.objid = pg_constraint.oid) AND (pg_depend.deptype = 'P'::"char") AND (pg_depend.refobjid IN ( SELECT pg_constraint_1.oid
                   FROM pg_constraint pg_constraint_1
                  WHERE (pg_constraint_1.contype = 'f'::"char"))))
         LIMIT 1) partitioned_parent_oids(parent_oid) ON (true))
  WHERE (pg_constraint.contype = 'f'::"char");


--
-- Name: postgres_index_bloat_estimates; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.postgres_index_bloat_estimates AS
 SELECT (((nspname)::text || '.'::text) || (idxname)::text) AS identifier,
    (
        CASE
            WHEN ((relpages)::double precision > est_pages_ff) THEN ((bs)::double precision * ((relpages)::double precision - est_pages_ff))
            ELSE (0)::double precision
        END)::bigint AS bloat_size_bytes
   FROM ( SELECT COALESCE(((1)::double precision + ceil((rows_hdr_pdg_stats.reltuples / floor((((((rows_hdr_pdg_stats.bs - (rows_hdr_pdg_stats.pageopqdata)::numeric) - (rows_hdr_pdg_stats.pagehdr)::numeric) * (rows_hdr_pdg_stats.fillfactor)::numeric))::double precision / ((100)::double precision * (((4)::numeric + rows_hdr_pdg_stats.nulldatahdrwidth))::double precision)))))), (0)::double precision) AS est_pages_ff,
            rows_hdr_pdg_stats.bs,
            rows_hdr_pdg_stats.nspname,
            rows_hdr_pdg_stats.tblname,
            rows_hdr_pdg_stats.idxname,
            rows_hdr_pdg_stats.relpages,
            rows_hdr_pdg_stats.is_na
           FROM ( SELECT rows_data_stats.maxalign,
                    rows_data_stats.bs,
                    rows_data_stats.nspname,
                    rows_data_stats.tblname,
                    rows_data_stats.idxname,
                    rows_data_stats.reltuples,
                    rows_data_stats.relpages,
                    rows_data_stats.idxoid,
                    rows_data_stats.fillfactor,
                    (((((((rows_data_stats.index_tuple_hdr_bm + rows_data_stats.maxalign) -
                        CASE
                            WHEN ((rows_data_stats.index_tuple_hdr_bm % rows_data_stats.maxalign) = 0) THEN rows_data_stats.maxalign
                            ELSE (rows_data_stats.index_tuple_hdr_bm % rows_data_stats.maxalign)
                        END))::double precision + rows_data_stats.nulldatawidth) + (rows_data_stats.maxalign)::double precision) - (
                        CASE
                            WHEN (rows_data_stats.nulldatawidth = (0)::double precision) THEN 0
                            WHEN (((rows_data_stats.nulldatawidth)::integer % rows_data_stats.maxalign) = 0) THEN rows_data_stats.maxalign
                            ELSE ((rows_data_stats.nulldatawidth)::integer % rows_data_stats.maxalign)
                        END)::double precision))::numeric AS nulldatahdrwidth,
                    rows_data_stats.pagehdr,
                    rows_data_stats.pageopqdata,
                    rows_data_stats.is_na
                   FROM ( SELECT n.nspname,
                            i.tblname,
                            i.idxname,
                            i.reltuples,
                            i.relpages,
                            i.idxoid,
                            i.fillfactor,
                            (current_setting('block_size'::text))::numeric AS bs,
                                CASE
                                    WHEN ((version() ~ 'mingw32'::text) OR (version() ~ '64-bit|x86_64|ppc64|ia64|amd64'::text)) THEN 8
                                    ELSE 4
                                END AS maxalign,
                            24 AS pagehdr,
                            16 AS pageopqdata,
                                CASE
                                    WHEN (max(COALESCE(s.null_frac, (0)::real)) = (0)::double precision) THEN 2
                                    ELSE (2 + (((32 + 8) - 1) / 8))
                                END AS index_tuple_hdr_bm,
                            sum((((1)::double precision - COALESCE(s.null_frac, (0)::real)) * (COALESCE(s.avg_width, 1024))::double precision)) AS nulldatawidth,
                            (max(
                                CASE
                                    WHEN (i.atttypid = ('name'::regtype)::oid) THEN 1
                                    ELSE 0
                                END) > 0) AS is_na
                           FROM ((( SELECT ct.relname AS tblname,
                                    ct.relnamespace,
                                    ic.idxname,
                                    ic.attpos,
                                    ic.indkey,
                                    ic.indkey[ic.attpos] AS indkey,
                                    ic.reltuples,
                                    ic.relpages,
                                    ic.tbloid,
                                    ic.idxoid,
                                    ic.fillfactor,
                                    COALESCE(a1.attnum, a2.attnum) AS attnum,
                                    COALESCE(a1.attname, a2.attname) AS attname,
                                    COALESCE(a1.atttypid, a2.atttypid) AS atttypid,
CASE
 WHEN (a1.attnum IS NULL) THEN ic.idxname
 ELSE ct.relname
END AS attrelname
                                   FROM (((( SELECT idx_data.idxname,
    idx_data.reltuples,
    idx_data.relpages,
    idx_data.tbloid,
    idx_data.idxoid,
    idx_data.fillfactor,
    idx_data.indkey,
    generate_series(1, (idx_data.indnatts)::integer) AS attpos
   FROM ( SELECT ci.relname AS idxname,
      ci.reltuples,
      ci.relpages,
      i_1.indrelid AS tbloid,
      i_1.indexrelid AS idxoid,
      COALESCE((("substring"(array_to_string(ci.reloptions, ' '::text), 'fillfactor=([0-9]+)'::text))::smallint)::integer, 90) AS fillfactor,
      i_1.indnatts,
      (string_to_array(textin(int2vectorout(i_1.indkey)), ' '::text))::integer[] AS indkey
     FROM (pg_index i_1
       JOIN pg_class ci ON ((ci.oid = i_1.indexrelid)))
    WHERE ((ci.relam = ( SELECT pg_am.oid
       FROM pg_am
      WHERE (pg_am.amname = 'btree'::name))) AND (ci.relpages > 0))) idx_data) ic
                                     JOIN pg_class ct ON ((ct.oid = ic.tbloid)))
                                     LEFT JOIN pg_attribute a1 ON (((ic.indkey[ic.attpos] <> 0) AND (a1.attrelid = ic.tbloid) AND (a1.attnum = ic.indkey[ic.attpos]))))
                                     LEFT JOIN pg_attribute a2 ON (((ic.indkey[ic.attpos] = 0) AND (a2.attrelid = ic.idxoid) AND (a2.attnum = ic.attpos))))) i(tblname, relnamespace, idxname, attpos, indkey, indkey_1, reltuples, relpages, tbloid, idxoid, fillfactor, attnum, attname, atttypid, attrelname)
                             JOIN pg_namespace n ON ((n.oid = i.relnamespace)))
                             JOIN pg_stats s ON (((s.schemaname = n.nspname) AND (s.tablename = i.attrelname) AND (s.attname = i.attname))))
                          GROUP BY n.nspname, i.tblname, i.idxname, i.reltuples, i.relpages, i.idxoid, i.fillfactor, (current_setting('block_size'::text))::numeric,
                                CASE
                                    WHEN ((version() ~ 'mingw32'::text) OR (version() ~ '64-bit|x86_64|ppc64|ia64|amd64'::text)) THEN 8
                                    ELSE 4
                                END, 24::integer, 16::integer) rows_data_stats) rows_hdr_pdg_stats) relation_stats
  WHERE ((nspname = ANY (ARRAY["current_schema"(), 'gitlab_partitions_dynamic'::name, 'gitlab_partitions_static'::name])) AND (NOT is_na))
  ORDER BY nspname, tblname, idxname;


--
-- Name: postgres_indexes; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.postgres_indexes AS
 SELECT (((pg_namespace.nspname)::text || '.'::text) || (i.relname)::text) AS identifier,
    pg_index.indexrelid,
    pg_namespace.nspname AS schema,
    i.relname AS name,
    pg_indexes.tablename,
    a.amname AS type,
    pg_index.indisunique AS "unique",
    pg_index.indisvalid AS valid_index,
    i.relispartition AS partitioned,
    pg_index.indisexclusion AS exclusion,
    (pg_index.indexprs IS NOT NULL) AS expression,
    (pg_index.indpred IS NOT NULL) AS partial,
    pg_indexes.indexdef AS definition,
    pg_relation_size((i.oid)::regclass) AS ondisk_size_bytes
   FROM ((((pg_index
     JOIN pg_class i ON ((i.oid = pg_index.indexrelid)))
     JOIN pg_namespace ON ((i.relnamespace = pg_namespace.oid)))
     JOIN pg_indexes ON (((i.relname = pg_indexes.indexname) AND (pg_namespace.nspname = pg_indexes.schemaname))))
     JOIN pg_am a ON ((i.relam = a.oid)))
  WHERE ((pg_namespace.nspname <> 'pg_catalog'::name) AND (pg_namespace.nspname = ANY (ARRAY["current_schema"(), 'gitlab_partitions_dynamic'::name, 'gitlab_partitions_static'::name])));


--
-- Name: postgres_partitioned_tables; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.postgres_partitioned_tables AS
 SELECT (((pg_namespace.nspname)::text || '.'::text) || (pg_class.relname)::text) AS identifier,
    pg_class.oid,
    pg_namespace.nspname AS schema,
    pg_class.relname AS name,
        CASE partitioned_tables.partstrat
            WHEN 'l'::"char" THEN 'list'::text
            WHEN 'r'::"char" THEN 'range'::text
            WHEN 'h'::"char" THEN 'hash'::text
            ELSE NULL::text
        END AS strategy,
    array_agg(pg_attribute.attname) AS key_columns
   FROM (((( SELECT pg_partitioned_table.partrelid,
            pg_partitioned_table.partstrat,
            unnest(pg_partitioned_table.partattrs) AS column_position
           FROM pg_partitioned_table) partitioned_tables
     JOIN pg_class ON ((partitioned_tables.partrelid = pg_class.oid)))
     JOIN pg_namespace ON ((pg_class.relnamespace = pg_namespace.oid)))
     JOIN pg_attribute ON (((pg_attribute.attrelid = pg_class.oid) AND (pg_attribute.attnum = partitioned_tables.column_position))))
  WHERE (pg_namespace.nspname = "current_schema"())
  GROUP BY (((pg_namespace.nspname)::text || '.'::text) || (pg_class.relname)::text), pg_class.oid, pg_namespace.nspname, pg_class.relname,
        CASE partitioned_tables.partstrat
            WHEN 'l'::"char" THEN 'list'::text
            WHEN 'r'::"char" THEN 'range'::text
            WHEN 'h'::"char" THEN 'hash'::text
            ELSE NULL::text
        END;


--
-- Name: postgres_partitions; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.postgres_partitions AS
 SELECT (((pg_namespace.nspname)::text || '.'::text) || (pg_class.relname)::text) AS identifier,
    pg_class.oid,
    pg_namespace.nspname AS schema,
    pg_class.relname AS name,
    (((parent_namespace.nspname)::text || '.'::text) || (parent_class.relname)::text) AS parent_identifier,
    pg_get_expr(pg_class.relpartbound, pg_inherits.inhrelid) AS condition,
    pg_inherits.inhdetachpending AS pending_detach
   FROM ((((pg_class
     JOIN pg_namespace ON ((pg_namespace.oid = pg_class.relnamespace)))
     JOIN pg_inherits ON ((pg_class.oid = pg_inherits.inhrelid)))
     JOIN pg_class parent_class ON ((pg_inherits.inhparent = parent_class.oid)))
     JOIN pg_namespace parent_namespace ON ((parent_class.relnamespace = parent_namespace.oid)))
  WHERE (pg_class.relispartition AND (pg_namespace.nspname = ANY (ARRAY["current_schema"(), 'gitlab_partitions_dynamic'::name, 'gitlab_partitions_static'::name])));


--
-- Name: postgres_sequences; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.postgres_sequences AS
 SELECT seq_pg_class.relname AS seq_name,
    dep_pg_class.relname AS table_name,
    pg_attribute.attname AS col_name,
    pg_sequence.seqmax AS seq_max,
    pg_sequence.seqmin AS seq_min,
    pg_sequence.seqstart AS seq_start,
    pg_sequence_last_value((pg_sequence.seqrelid)::regclass) AS last_value
   FROM ((((pg_class seq_pg_class
     JOIN pg_sequence ON ((seq_pg_class.oid = pg_sequence.seqrelid)))
     LEFT JOIN pg_depend ON (((seq_pg_class.oid = pg_depend.objid) AND (pg_depend.classid = ('pg_class'::regclass)::oid) AND (pg_depend.refclassid = ('pg_class'::regclass)::oid))))
     LEFT JOIN pg_class dep_pg_class ON ((pg_depend.refobjid = dep_pg_class.oid)))
     LEFT JOIN pg_attribute ON (((dep_pg_class.oid = pg_attribute.attrelid) AND (pg_depend.refobjsubid = pg_attribute.attnum))))
  WHERE (seq_pg_class.relkind = 'S'::"char");


--
-- Name: postgres_table_sizes; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.postgres_table_sizes AS
 SELECT (((schemaname)::text || '.'::text) || (relname)::text) AS identifier,
    schemaname AS schema_name,
    relname AS table_name,
    pg_size_pretty(total_bytes) AS total_size,
    pg_size_pretty(table_bytes) AS table_size,
    pg_size_pretty(index_bytes) AS index_size,
    pg_size_pretty(toast_bytes) AS toast_size,
    pg_size_pretty((((total_bytes - table_bytes) - index_bytes) - toast_bytes)) AS auxiliary_size,
    total_bytes AS size_in_bytes
   FROM ( SELECT pg_stat_user_tables.schemaname,
            pg_stat_user_tables.relname,
            pg_total_relation_size((((quote_ident((pg_stat_user_tables.schemaname)::text) || '.'::text) || quote_ident((pg_stat_user_tables.relname)::text)))::regclass) AS total_bytes,
            pg_relation_size((((quote_ident((pg_stat_user_tables.schemaname)::text) || '.'::text) || quote_ident((pg_stat_user_tables.relname)::text)))::regclass) AS table_bytes,
            pg_indexes_size((((quote_ident((pg_stat_user_tables.schemaname)::text) || '.'::text) || quote_ident((pg_stat_user_tables.relname)::text)))::regclass) AS index_bytes,
            ((pg_total_relation_size((((quote_ident((pg_stat_user_tables.schemaname)::text) || '.'::text) || quote_ident((pg_stat_user_tables.relname)::text)))::regclass) - pg_relation_size((((quote_ident((pg_stat_user_tables.schemaname)::text) || '.'::text) || quote_ident((pg_stat_user_tables.relname)::text)))::regclass)) - pg_indexes_size((((quote_ident((pg_stat_user_tables.schemaname)::text) || '.'::text) || quote_ident((pg_stat_user_tables.relname)::text)))::regclass)) AS toast_bytes
           FROM pg_stat_user_tables
          WHERE (pg_total_relation_size((((quote_ident((pg_stat_user_tables.schemaname)::text) || '.'::text) || quote_ident((pg_stat_user_tables.relname)::text)))::regclass) IS NOT NULL)) t
  ORDER BY total_bytes DESC;


--
-- Name: postgres_triggers; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.postgres_triggers AS
 SELECT concat(nsp.nspname, '.', rel.relname, '.', trgr.tgname) AS identifier,
    trgr.tgname AS trigger_name,
    rel.relname AS table_name,
    nsp.nspname AS schema_name,
    proc.proname AS function_name
   FROM (((pg_trigger trgr
     JOIN pg_class rel ON ((trgr.tgrelid = rel.oid)))
     JOIN pg_namespace nsp ON ((nsp.oid = rel.relnamespace)))
     LEFT JOIN pg_proc proc ON ((trgr.tgfoid = proc.oid)))
  WHERE ((NOT trgr.tgisinternal) AND (nsp.nspname <> ALL (ARRAY['information_schema'::name, 'pg_catalog'::name, 'pg_toast'::name])));


--
-- Name: project_authorizations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.project_authorizations (
    user_id bigint NOT NULL,
    project_id bigint NOT NULL,
    access_level integer NOT NULL,
    is_unique boolean
);


--
-- Name: project_authorizations_for_migration; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.project_authorizations_for_migration (
    user_id bigint NOT NULL,
    project_id bigint NOT NULL,
    access_level smallint NOT NULL
);


--
-- Name: project_settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.project_settings (
    project_id bigint NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    push_rule_id bigint,
    show_default_award_emojis boolean DEFAULT true,
    allow_merge_on_skipped_pipeline boolean,
    squash_option smallint DEFAULT 3,
    has_confluence boolean DEFAULT false NOT NULL,
    has_vulnerabilities boolean DEFAULT false NOT NULL,
    prevent_merge_without_jira_issue boolean DEFAULT false NOT NULL,
    cve_id_request_enabled boolean DEFAULT true NOT NULL,
    mr_default_target_self boolean DEFAULT false NOT NULL,
    previous_default_branch text,
    warn_about_potentially_unwanted_characters boolean DEFAULT true NOT NULL,
    merge_commit_template text,
    has_shimo boolean DEFAULT false NOT NULL,
    squash_commit_template text,
    legacy_open_source_license_available boolean DEFAULT true NOT NULL,
    target_platforms character varying[] DEFAULT '{}'::character varying[] NOT NULL,
    enforce_auth_checks_on_uploads boolean DEFAULT true NOT NULL,
    selective_code_owner_removals boolean DEFAULT false NOT NULL,
    issue_branch_template text,
    show_diff_preview_in_email boolean DEFAULT true NOT NULL,
    suggested_reviewers_enabled boolean DEFAULT false NOT NULL,
    only_allow_merge_if_all_status_checks_passed boolean DEFAULT false NOT NULL,
    mirror_branch_regex text,
    allow_pipeline_trigger_approve_deployment boolean DEFAULT false NOT NULL,
    emails_enabled boolean DEFAULT true NOT NULL,
    pages_unique_domain_enabled boolean DEFAULT false NOT NULL,
    pages_unique_domain text,
    runner_registration_enabled boolean DEFAULT true,
    product_analytics_instrumentation_key text,
    product_analytics_data_collector_host text,
    cube_api_base_url text,
    encrypted_cube_api_key bytea,
    encrypted_cube_api_key_iv bytea,
    encrypted_product_analytics_configurator_connection_string bytea,
    encrypted_product_analytics_configurator_connection_string_iv bytea,
    pages_multiple_versions_enabled boolean DEFAULT false NOT NULL,
    allow_merge_without_pipeline boolean DEFAULT false NOT NULL,
    duo_features_enabled boolean DEFAULT true NOT NULL,
    require_reauthentication_to_approve boolean,
    observability_alerts_enabled boolean DEFAULT true NOT NULL,
    spp_repository_pipeline_access boolean DEFAULT true,
    max_number_of_vulnerabilities integer,
    pages_primary_domain text,
    extended_prat_expiry_webhooks_execute boolean DEFAULT false NOT NULL,
    merge_request_title_regex text,
    protect_merge_request_pipelines boolean DEFAULT true NOT NULL,
    auto_duo_code_review_enabled boolean,
    model_prompt_cache_enabled boolean,
    web_based_commit_signing_enabled boolean DEFAULT false NOT NULL,
    duo_context_exclusion_settings jsonb DEFAULT '{}'::jsonb NOT NULL,
    merge_request_title_regex_description text,
    duo_remote_flows_enabled boolean,
    duo_foundational_flows_enabled boolean,
    duo_sast_fp_detection_enabled boolean DEFAULT false NOT NULL,
    duo_sast_vr_workflow_enabled boolean DEFAULT false NOT NULL,
    automatic_rebase_enabled boolean DEFAULT false NOT NULL,
    duo_secret_detection_fp_enabled boolean DEFAULT false NOT NULL,
    reviewer_assignment_strategy smallint DEFAULT 0 NOT NULL,
    pipeline_execution_policy_bot_access_enabled boolean DEFAULT false NOT NULL,
    pipeline_execution_policy_bot_access_file_patterns text[] DEFAULT '{}'::text[],
    pipeline_execution_policy_bot_access_group_id bigint,
    security_policy_pipeline_must_succeed boolean DEFAULT false NOT NULL,
    mr_default_title_template text,
    tool_approval_for_session_enabled boolean,
    dap_session_tracking_enabled boolean DEFAULT false NOT NULL,
    duo_dependency_bump_breaking_changes_enabled boolean DEFAULT false NOT NULL,
    ai_audit_events_storage_enabled boolean DEFAULT false NOT NULL,
    duo_dependency_bump_breaking_changes_enabled_by_id bigint,
    feature_flags_minimum_role smallint DEFAULT 2 NOT NULL,
    duo_vulnerability_context_analysis_enabled boolean DEFAULT false NOT NULL,
    CONSTRAINT check_1a30456322 CHECK ((char_length(pages_unique_domain) <= 63)),
    CONSTRAINT check_237486989c CHECK ((char_length(merge_request_title_regex_description) <= 255)),
    CONSTRAINT check_3a03e7557a CHECK ((char_length(previous_default_branch) <= 4096)),
    CONSTRAINT check_3ca5cbffe6 CHECK ((char_length(issue_branch_template) <= 255)),
    CONSTRAINT check_4b142e71f3 CHECK ((char_length(product_analytics_data_collector_host) <= 255)),
    CONSTRAINT check_67292e4b99 CHECK ((char_length(mirror_branch_regex) <= 255)),
    CONSTRAINT check_999e5f0aaa CHECK ((char_length(pages_primary_domain) <= 255)),
    CONSTRAINT check_acb7fad2f9 CHECK ((char_length(product_analytics_instrumentation_key) <= 255)),
    CONSTRAINT check_b09644994b CHECK ((char_length(squash_commit_template) <= 500)),
    CONSTRAINT check_bde223416c CHECK ((show_default_award_emojis IS NOT NULL)),
    CONSTRAINT check_e1b8627f0b CHECK ((char_length(mr_default_title_template) <= 100)),
    CONSTRAINT check_eaf7cfb6a7 CHECK ((char_length(merge_commit_template) <= 500)),
    CONSTRAINT check_ee0d751d5c CHECK ((char_length(merge_request_title_regex) <= 255)),
    CONSTRAINT check_f9df7bcee2 CHECK ((char_length(cube_api_base_url) <= 512)),
    CONSTRAINT check_project_settings_pep_bot_access_file_patterns_size CHECK ((cardinality(pipeline_execution_policy_bot_access_file_patterns) <= 20))
);


--
-- Name: projects_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.projects_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: projects_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.projects_id_seq OWNED BY public.projects.id;


--
-- Name: projects_sync_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.projects_sync_events (
    id bigint NOT NULL,
    project_id bigint NOT NULL
);


--
-- Name: projects_sync_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.projects_sync_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: projects_sync_events_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.projects_sync_events_id_seq OWNED BY public.projects_sync_events.id;


--
-- Name: push_rules; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.push_rules (
    id bigint NOT NULL,
    commit_message_regex character varying,
    deny_delete_tag boolean,
    project_id bigint,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    author_email_regex character varying,
    member_check boolean DEFAULT false NOT NULL,
    file_name_regex character varying,
    is_sample boolean DEFAULT false,
    max_file_size integer DEFAULT 0 NOT NULL,
    prevent_secrets boolean DEFAULT false NOT NULL,
    branch_name_regex character varying,
    reject_unsigned_commits boolean,
    commit_committer_check boolean,
    regexp_uses_re2 boolean DEFAULT true,
    commit_message_negative_regex character varying,
    reject_non_dco_commits boolean,
    commit_committer_name_check boolean DEFAULT false NOT NULL,
    organization_id bigint,
    CONSTRAINT author_email_regex_size_constraint CHECK ((char_length((author_email_regex)::text) <= 511)),
    CONSTRAINT branch_name_regex_size_constraint CHECK ((char_length((branch_name_regex)::text) <= 511)),
    CONSTRAINT commit_message_negative_regex_size_constraint CHECK ((char_length((commit_message_negative_regex)::text) <= 2047)),
    CONSTRAINT commit_message_regex_size_constraint CHECK ((char_length((commit_message_regex)::text) <= 511)),
    CONSTRAINT file_name_regex_size_constraint CHECK ((char_length((file_name_regex)::text) <= 511))
);


--
-- Name: push_rules_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.push_rules_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: push_rules_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.push_rules_id_seq OWNED BY public.push_rules.id;


--
-- Name: reviews; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.reviews (
    id bigint NOT NULL,
    author_id bigint,
    merge_request_id bigint NOT NULL,
    project_id bigint NOT NULL,
    created_at timestamp with time zone NOT NULL
);


--
-- Name: reviews_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.reviews_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: reviews_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.reviews_id_seq OWNED BY public.reviews.id;


--
-- Name: saml_providers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.saml_providers (
    id bigint NOT NULL,
    group_id bigint NOT NULL,
    enabled boolean NOT NULL,
    certificate_fingerprint character varying NOT NULL,
    sso_url character varying NOT NULL,
    enforced_sso boolean DEFAULT false NOT NULL,
    enforced_group_managed_accounts boolean DEFAULT false NOT NULL,
    prohibited_outer_forks boolean DEFAULT true NOT NULL,
    default_membership_role smallint DEFAULT 10 NOT NULL,
    git_check_enforced boolean DEFAULT false NOT NULL,
    member_role_id bigint,
    disable_password_authentication_for_enterprise_users boolean DEFAULT false
);


--
-- Name: saml_providers_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.saml_providers_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: saml_providers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.saml_providers_id_seq OWNED BY public.saml_providers.id;


--
-- Name: sent_notifications_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.sent_notifications_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: shared_audit_event_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.shared_audit_event_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: sprints; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sprints (
    id bigint NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    start_date date,
    due_date date,
    group_id bigint,
    iid integer NOT NULL,
    cached_markdown_version integer,
    title text,
    title_html text,
    description text,
    description_html text,
    state_enum smallint DEFAULT 1 NOT NULL,
    iterations_cadence_id bigint,
    sequence integer,
    CONSTRAINT check_73910a3b6c CHECK ((group_id IS NOT NULL)),
    CONSTRAINT sprints_title CHECK ((char_length(title) <= 255))
);


--
-- Name: sprints_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.sprints_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: sprints_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.sprints_id_seq OWNED BY public.sprints.id;


--
-- Name: subscription_add_on_purchases; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.subscription_add_on_purchases (
    id bigint NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    subscription_add_on_id bigint,
    namespace_id bigint,
    quantity integer NOT NULL,
    expires_on date NOT NULL,
    purchase_xid text NOT NULL,
    last_assigned_users_refreshed_at timestamp with time zone,
    trial boolean DEFAULT false NOT NULL,
    started_at date,
    organization_id bigint NOT NULL,
    subscription_add_on_uid smallint,
    CONSTRAINT check_3313c4d200 CHECK ((char_length(purchase_xid) <= 255)),
    CONSTRAINT check_c4de34843d CHECK ((subscription_add_on_uid IS NOT NULL)),
    CONSTRAINT check_d79ce199b3 CHECK ((started_at IS NOT NULL))
);


--
-- Name: subscription_add_on_purchases_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.subscription_add_on_purchases_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: subscription_add_on_purchases_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.subscription_add_on_purchases_id_seq OWNED BY public.subscription_add_on_purchases.id;


--
-- Name: subscription_user_add_on_assignments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.subscription_user_add_on_assignments (
    id bigint NOT NULL,
    add_on_purchase_id bigint NOT NULL,
    user_id bigint NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    organization_id bigint,
    CONSTRAINT check_7d21f9cebf CHECK ((organization_id IS NOT NULL))
);


--
-- Name: subscription_user_add_on_assignments_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.subscription_user_add_on_assignments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: subscription_user_add_on_assignments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.subscription_user_add_on_assignments_id_seq OWNED BY public.subscription_user_add_on_assignments.id;


--
-- Name: todos; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.todos (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    project_id bigint,
    target_id bigint,
    target_type character varying NOT NULL,
    author_id bigint NOT NULL,
    action integer NOT NULL,
    state character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    commit_id character varying,
    group_id bigint,
    resolved_by_action smallint,
    note_id bigint,
    snoozed_until timestamp with time zone,
    organization_id bigint,
    CONSTRAINT check_3c13ed1c7a CHECK ((num_nonnulls(group_id, organization_id, project_id) = 1))
);


--
-- Name: todos_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.todos_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: todos_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.todos_id_seq OWNED BY public.todos.id;


--
-- Name: user_details; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_details (
    user_id bigint NOT NULL,
    job_title character varying(200) DEFAULT ''::character varying NOT NULL,
    bio character varying(255) DEFAULT ''::character varying NOT NULL,
    webauthn_xid text,
    provisioned_by_group_id bigint,
    pronouns text,
    pronunciation text,
    phone text,
    linkedin text DEFAULT ''::text NOT NULL,
    twitter text DEFAULT ''::text NOT NULL,
    website_url text DEFAULT ''::text NOT NULL,
    location text DEFAULT ''::text NOT NULL,
    password_last_changed_at timestamp with time zone DEFAULT now() NOT NULL,
    discord text DEFAULT ''::text NOT NULL,
    enterprise_group_id bigint,
    enterprise_group_associated_at timestamp with time zone,
    email_reset_offered_at timestamp with time zone,
    mastodon text DEFAULT ''::text NOT NULL,
    project_authorizations_recalculated_at timestamp with time zone DEFAULT '2010-01-01 00:00:00+00'::timestamp with time zone NOT NULL,
    onboarding_status jsonb DEFAULT '{}'::jsonb NOT NULL,
    bluesky text DEFAULT ''::text NOT NULL,
    bot_namespace_id bigint,
    orcid text DEFAULT ''::text NOT NULL,
    github text DEFAULT ''::text NOT NULL,
    email_otp text,
    email_otp_last_sent_to text,
    email_otp_last_sent_at timestamp with time zone,
    email_otp_required_after timestamp with time zone,
    company text DEFAULT ''::text NOT NULL,
    provisioned_by_project_id bigint,
    CONSTRAINT check_18a53381cd CHECK ((char_length(bluesky) <= 256)),
    CONSTRAINT check_245664af82 CHECK ((char_length(webauthn_xid) <= 100)),
    CONSTRAINT check_466a25be35 CHECK ((char_length(twitter) <= 500)),
    CONSTRAINT check_4925cf9fd2 CHECK ((char_length(email_otp_last_sent_to) <= 511)),
    CONSTRAINT check_4ef1de1a15 CHECK ((char_length(discord) <= 500)),
    CONSTRAINT check_7d6489f8f3 CHECK ((char_length(linkedin) <= 500)),
    CONSTRAINT check_7fe2044093 CHECK ((char_length(website_url) <= 500)),
    CONSTRAINT check_8a7fcf8a60 CHECK ((char_length(location) <= 500)),
    CONSTRAINT check_99b0365865 CHECK ((char_length(orcid) <= 256)),
    CONSTRAINT check_a73b398c60 CHECK ((char_length(phone) <= 50)),
    CONSTRAINT check_bbe110f371 CHECK ((char_length(github) <= 500)),
    CONSTRAINT check_ec514a06ad CHECK ((char_length(email_otp) <= 64)),
    CONSTRAINT check_eeeaf8d4f0 CHECK ((char_length(pronouns) <= 50)),
    CONSTRAINT check_f1a8a05b9a CHECK ((char_length(mastodon) <= 500)),
    CONSTRAINT check_f932ed37db CHECK ((char_length(pronunciation) <= 255)),
    CONSTRAINT check_user_details_provisioned_by_mutually_exclusive CHECK ((num_nonnulls(provisioned_by_group_id, provisioned_by_project_id) <= 1))
);


--
-- Name: COLUMN user_details.phone; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_details.phone IS 'JiHu-specific column';


--
-- Name: COLUMN user_details.password_last_changed_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_details.password_last_changed_at IS 'JiHu-specific column';


--
-- Name: COLUMN user_details.email_otp; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.user_details.email_otp IS 'SHA256 hash (64 hex characters)';


--
-- Name: user_details_user_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_details_user_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_details_user_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_details_user_id_seq OWNED BY public.user_details.user_id;


--
-- Name: user_preferences; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_preferences (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    issue_notes_filter smallint DEFAULT 0 NOT NULL,
    merge_request_notes_filter smallint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    epics_sort character varying,
    roadmap_epics_state integer,
    epic_notes_filter smallint DEFAULT 0 NOT NULL,
    issues_sort character varying,
    merge_requests_sort character varying,
    roadmaps_sort character varying,
    first_day_of_week integer,
    timezone character varying,
    time_display_relative boolean DEFAULT true,
    projects_sort character varying(64),
    show_whitespace_in_diffs boolean DEFAULT true NOT NULL,
    sourcegraph_enabled boolean,
    setup_for_company boolean,
    render_whitespace_in_code boolean DEFAULT false,
    tab_width smallint DEFAULT 8,
    view_diffs_file_by_file boolean DEFAULT false NOT NULL,
    gitpod_enabled boolean DEFAULT false NOT NULL,
    markdown_surround_selection boolean DEFAULT true NOT NULL,
    diffs_deletion_color text,
    diffs_addition_color text,
    markdown_automatic_lists boolean DEFAULT true NOT NULL,
    use_new_navigation boolean,
    achievements_enabled boolean DEFAULT true NOT NULL,
    pinned_nav_items jsonb DEFAULT '{}'::jsonb NOT NULL,
    pass_user_identities_to_ci_jwt boolean DEFAULT false NOT NULL,
    enabled_following boolean DEFAULT true NOT NULL,
    visibility_pipeline_id_type smallint DEFAULT 0 NOT NULL,
    project_shortcut_buttons boolean DEFAULT true NOT NULL,
    enabled_zoekt boolean DEFAULT true NOT NULL,
    keyboard_shortcuts_enabled boolean DEFAULT true NOT NULL,
    time_display_format smallint DEFAULT 0 NOT NULL,
    early_access_program_participant boolean DEFAULT false NOT NULL,
    early_access_program_tracking boolean DEFAULT false NOT NULL,
    extensions_marketplace_opt_in_status smallint DEFAULT 0 NOT NULL,
    organization_groups_projects_sort text,
    organization_groups_projects_display smallint DEFAULT 1 NOT NULL,
    dpop_enabled boolean DEFAULT false NOT NULL,
    use_work_items_view boolean DEFAULT false NOT NULL,
    text_editor_type smallint DEFAULT 2 NOT NULL,
    merge_request_dashboard_list_type smallint DEFAULT 0 NOT NULL,
    extensions_marketplace_opt_in_url text,
    dark_color_scheme_id smallint,
    work_items_display_settings jsonb DEFAULT '{}'::jsonb NOT NULL,
    default_duo_add_on_assignment_id bigint,
    markdown_maintain_indentation boolean DEFAULT false NOT NULL,
    merge_request_dashboard_show_drafts boolean DEFAULT true NOT NULL,
    duo_default_namespace_id bigint,
    policy_advanced_editor boolean DEFAULT false NOT NULL,
    early_access_studio_participant boolean DEFAULT false NOT NULL,
    wiki_use_auto_commit_message boolean DEFAULT false NOT NULL,
    orbit_settings jsonb DEFAULT '{}'::jsonb NOT NULL,
    knowledge_graph_governing_namespace_id bigint,
    emoji_autocomplete_enabled boolean DEFAULT true NOT NULL,
    CONSTRAINT check_1d670edc68 CHECK ((time_display_relative IS NOT NULL)),
    CONSTRAINT check_89bf269f41 CHECK ((char_length(diffs_deletion_color) <= 7)),
    CONSTRAINT check_9b50d9f942 CHECK ((char_length(extensions_marketplace_opt_in_url) <= 512)),
    CONSTRAINT check_b1306f8875 CHECK ((char_length(organization_groups_projects_sort) <= 64)),
    CONSTRAINT check_b22446f91a CHECK ((render_whitespace_in_code IS NOT NULL)),
    CONSTRAINT check_d07ccd35f7 CHECK ((char_length(diffs_addition_color) <= 7)),
    CONSTRAINT check_d3248b1b9c CHECK ((tab_width IS NOT NULL))
);


--
-- Name: user_preferences_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_preferences_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_preferences_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_preferences_id_seq OWNED BY public.user_preferences.id;


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.users_id_seq
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
-- Name: virtual_registries_container_cache_remote_entries_iid_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.virtual_registries_container_cache_remote_entries_iid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: virtual_registries_packages_maven_cache_local_entries_iid_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.virtual_registries_packages_maven_cache_local_entries_iid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: virtual_registries_packages_maven_cache_remote_entries_iid_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.virtual_registries_packages_maven_cache_remote_entries_iid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: virtual_registries_packages_npm_cache_local_entries_iid_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.virtual_registries_packages_npm_cache_local_entries_iid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: virtual_registries_packages_npm_cache_remote_entries_iid_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.virtual_registries_packages_npm_cache_remote_entries_iid_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: work_item_parent_links; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.work_item_parent_links (
    id bigint NOT NULL,
    work_item_id bigint NOT NULL,
    work_item_parent_id bigint NOT NULL,
    relative_position integer,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    namespace_id bigint,
    CONSTRAINT check_e9c0111985 CHECK ((namespace_id IS NOT NULL))
);


--
-- Name: work_item_parent_links_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.work_item_parent_links_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: work_item_parent_links_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.work_item_parent_links_id_seq OWNED BY public.work_item_parent_links.id;


--
-- Name: work_item_positions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.work_item_positions (
    work_item_id bigint NOT NULL,
    namespace_id bigint NOT NULL,
    relative_position bigint,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    relative_positioning_namespace_id bigint
);


--
-- Name: work_item_transitions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.work_item_transitions (
    work_item_id bigint NOT NULL,
    namespace_id bigint NOT NULL,
    moved_to_id bigint,
    duplicated_to_id bigint,
    promoted_to_epic_id bigint
);


--
-- Name: abuse_reports id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.abuse_reports ALTER COLUMN id SET DEFAULT nextval('public.abuse_reports_id_seq'::regclass);


--
-- Name: award_emoji id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.award_emoji ALTER COLUMN id SET DEFAULT nextval('public.award_emoji_id_seq'::regclass);


--
-- Name: catalog_resources id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.catalog_resources ALTER COLUMN id SET DEFAULT nextval('public.catalog_resources_id_seq'::regclass);


--
-- Name: compliance_management_frameworks id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.compliance_management_frameworks ALTER COLUMN id SET DEFAULT nextval('public.compliance_management_frameworks_id_seq'::regclass);


--
-- Name: emails id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.emails ALTER COLUMN id SET DEFAULT nextval('public.emails_id_seq'::regclass);


--
-- Name: epics id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.epics ALTER COLUMN id SET DEFAULT nextval('public.epics_id_seq'::regclass);


--
-- Name: events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.events ALTER COLUMN id SET DEFAULT nextval('public.events_id_seq'::regclass);


--
-- Name: identities id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.identities ALTER COLUMN id SET DEFAULT nextval('public.identities_id_seq'::regclass);


--
-- Name: issues id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issues ALTER COLUMN id SET DEFAULT nextval('public.issues_id_seq'::regclass);


--
-- Name: iterations_cadences id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.iterations_cadences ALTER COLUMN id SET DEFAULT nextval('public.iterations_cadences_id_seq'::regclass);


--
-- Name: loose_foreign_keys_deleted_records id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.loose_foreign_keys_deleted_records ALTER COLUMN id SET DEFAULT nextval('public.loose_foreign_keys_deleted_records_id_seq'::regclass);


--
-- Name: member_roles id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.member_roles ALTER COLUMN id SET DEFAULT nextval('public.member_roles_id_seq'::regclass);


--
-- Name: members id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.members ALTER COLUMN id SET DEFAULT nextval('public.members_id_seq'::regclass);


--
-- Name: merge_request_diffs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merge_request_diffs ALTER COLUMN id SET DEFAULT nextval('public.merge_request_diffs_id_seq'::regclass);


--
-- Name: merge_requests id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merge_requests ALTER COLUMN id SET DEFAULT nextval('public.merge_requests_id_seq'::regclass);


--
-- Name: milestones id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.milestones ALTER COLUMN id SET DEFAULT nextval('public.milestones_id_seq'::regclass);


--
-- Name: namespaces id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.namespaces ALTER COLUMN id SET DEFAULT nextval('public.namespaces_id_seq'::regclass);


--
-- Name: namespaces_sync_events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.namespaces_sync_events ALTER COLUMN id SET DEFAULT nextval('public.namespaces_sync_events_id_seq'::regclass);


--
-- Name: notes id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notes ALTER COLUMN id SET DEFAULT nextval('public.notes_id_seq'::regclass);


--
-- Name: organizations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.organizations ALTER COLUMN id SET DEFAULT nextval('public.organizations_id_seq'::regclass);


--
-- Name: p_catalog_resource_sync_events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.p_catalog_resource_sync_events ALTER COLUMN id SET DEFAULT nextval('public.p_catalog_resource_sync_events_id_seq'::regclass);


--
-- Name: personal_access_tokens id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.personal_access_tokens ALTER COLUMN id SET DEFAULT nextval('public.personal_access_tokens_id_seq'::regclass);


--
-- Name: projects id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.projects ALTER COLUMN id SET DEFAULT nextval('public.projects_id_seq'::regclass);


--
-- Name: projects_sync_events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.projects_sync_events ALTER COLUMN id SET DEFAULT nextval('public.projects_sync_events_id_seq'::regclass);


--
-- Name: push_rules id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.push_rules ALTER COLUMN id SET DEFAULT nextval('public.push_rules_id_seq'::regclass);


--
-- Name: reviews id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviews ALTER COLUMN id SET DEFAULT nextval('public.reviews_id_seq'::regclass);


--
-- Name: saml_providers id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.saml_providers ALTER COLUMN id SET DEFAULT nextval('public.saml_providers_id_seq'::regclass);


--
-- Name: sprints id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sprints ALTER COLUMN id SET DEFAULT nextval('public.sprints_id_seq'::regclass);


--
-- Name: subscription_add_on_purchases id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.subscription_add_on_purchases ALTER COLUMN id SET DEFAULT nextval('public.subscription_add_on_purchases_id_seq'::regclass);


--
-- Name: subscription_user_add_on_assignments id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.subscription_user_add_on_assignments ALTER COLUMN id SET DEFAULT nextval('public.subscription_user_add_on_assignments_id_seq'::regclass);


--
-- Name: todos id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.todos ALTER COLUMN id SET DEFAULT nextval('public.todos_id_seq'::regclass);


--
-- Name: user_details user_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_details ALTER COLUMN user_id SET DEFAULT nextval('public.user_details_user_id_seq'::regclass);


--
-- Name: user_preferences id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_preferences ALTER COLUMN id SET DEFAULT nextval('public.user_preferences_id_seq'::regclass);


--
-- Name: users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users ALTER COLUMN id SET DEFAULT nextval('public.users_id_seq'::regclass);


--
-- Name: work_item_parent_links id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.work_item_parent_links ALTER COLUMN id SET DEFAULT nextval('public.work_item_parent_links_id_seq'::regclass);


--
-- Name: abuse_reports abuse_reports_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.abuse_reports
    ADD CONSTRAINT abuse_reports_pkey PRIMARY KEY (id);


--
-- Name: award_emoji award_emoji_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.award_emoji
    ADD CONSTRAINT award_emoji_pkey PRIMARY KEY (id);


--
-- Name: catalog_resources catalog_resources_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.catalog_resources
    ADD CONSTRAINT catalog_resources_pkey PRIMARY KEY (id);


--
-- Name: push_rules check_1d23f0a102; Type: CHECK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.push_rules
    ADD CONSTRAINT check_1d23f0a102 CHECK ((project_id IS NOT NULL)) NOT VALID;


--
-- Name: user_details check_3b9aec5742; Type: CHECK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.user_details
    ADD CONSTRAINT check_3b9aec5742 CHECK ((char_length(company) <= 500)) NOT VALID;


--
-- Name: namespace_settings check_939de199cb; Type: CHECK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.namespace_settings
    ADD CONSTRAINT check_939de199cb CHECK ((char_length(ai_custom_instructions) <= 2000)) NOT VALID;


--
-- Name: abuse_reports check_95e5f0c300; Type: CHECK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.abuse_reports
    ADD CONSTRAINT check_95e5f0c300 CHECK ((char_length(message) <= 2048)) NOT VALID;


--
-- Name: sprints check_ccd8a1eae0; Type: CHECK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.sprints
    ADD CONSTRAINT check_ccd8a1eae0 CHECK ((start_date IS NOT NULL)) NOT VALID;


--
-- Name: compliance_management_frameworks check_dbba6ffa2a; Type: CHECK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.compliance_management_frameworks
    ADD CONSTRAINT check_dbba6ffa2a CHECK ((char_length(template_id) <= 255)) NOT VALID;


--
-- Name: sprints check_df3816aed7; Type: CHECK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.sprints
    ADD CONSTRAINT check_df3816aed7 CHECK ((due_date IS NOT NULL)) NOT VALID;


--
-- Name: compliance_management_frameworks compliance_management_frameworks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.compliance_management_frameworks
    ADD CONSTRAINT compliance_management_frameworks_pkey PRIMARY KEY (id);


--
-- Name: namespace_settings default_branch_protection_defaults_size_constraint; Type: CHECK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.namespace_settings
    ADD CONSTRAINT default_branch_protection_defaults_size_constraint CHECK ((octet_length((default_branch_protection_defaults)::text) <= 1024)) NOT VALID;


--
-- Name: emails emails_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.emails
    ADD CONSTRAINT emails_pkey PRIMARY KEY (id);


--
-- Name: epics epics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.epics
    ADD CONSTRAINT epics_pkey PRIMARY KEY (id);


--
-- Name: events events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.events
    ADD CONSTRAINT events_pkey PRIMARY KEY (id);


--
-- Name: identities identities_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.identities
    ADD CONSTRAINT identities_pkey PRIMARY KEY (id);


--
-- Name: issue_assignees issue_assignees_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_assignees
    ADD CONSTRAINT issue_assignees_pkey PRIMARY KEY (issue_id, user_id);


--
-- Name: issues issues_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issues
    ADD CONSTRAINT issues_pkey PRIMARY KEY (id);


--
-- Name: sprints iteration_start_and_due_date_iterations_cadence_id_constraint; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sprints
    ADD CONSTRAINT iteration_start_and_due_date_iterations_cadence_id_constraint EXCLUDE USING gist (iterations_cadence_id WITH =, daterange(start_date, due_date, '[]'::text) WITH &&) WHERE ((group_id IS NOT NULL)) DEFERRABLE INITIALLY DEFERRED;


--
-- Name: iterations_cadences iterations_cadences_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.iterations_cadences
    ADD CONSTRAINT iterations_cadences_pkey PRIMARY KEY (id);


--
-- Name: loose_foreign_keys_deleted_records loose_foreign_keys_deleted_records_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.loose_foreign_keys_deleted_records
    ADD CONSTRAINT loose_foreign_keys_deleted_records_pkey PRIMARY KEY (partition, id);


--
-- Name: member_roles member_roles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.member_roles
    ADD CONSTRAINT member_roles_pkey PRIMARY KEY (id);


--
-- Name: members members_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.members
    ADD CONSTRAINT members_pkey PRIMARY KEY (id);


--
-- Name: merge_request_diffs merge_request_diffs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merge_request_diffs
    ADD CONSTRAINT merge_request_diffs_pkey PRIMARY KEY (id);


--
-- Name: merge_requests merge_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merge_requests
    ADD CONSTRAINT merge_requests_pkey PRIMARY KEY (id);


--
-- Name: milestones milestones_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.milestones
    ADD CONSTRAINT milestones_pkey PRIMARY KEY (id);


--
-- Name: namespace_details namespace_details_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.namespace_details
    ADD CONSTRAINT namespace_details_pkey PRIMARY KEY (namespace_id);


--
-- Name: namespace_settings namespace_settings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.namespace_settings
    ADD CONSTRAINT namespace_settings_pkey PRIMARY KEY (namespace_id);


--
-- Name: namespaces namespaces_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.namespaces
    ADD CONSTRAINT namespaces_pkey PRIMARY KEY (id);


--
-- Name: namespaces_sync_events namespaces_sync_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.namespaces_sync_events
    ADD CONSTRAINT namespaces_sync_events_pkey PRIMARY KEY (id);


--
-- Name: notes notes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notes
    ADD CONSTRAINT notes_pkey PRIMARY KEY (id);


--
-- Name: organizations organizations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.organizations
    ADD CONSTRAINT organizations_pkey PRIMARY KEY (id);


--
-- Name: p_catalog_resource_sync_events p_catalog_resource_sync_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.p_catalog_resource_sync_events
    ADD CONSTRAINT p_catalog_resource_sync_events_pkey PRIMARY KEY (id, partition_id);


--
-- Name: personal_access_tokens personal_access_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.personal_access_tokens
    ADD CONSTRAINT personal_access_tokens_pkey PRIMARY KEY (id);


--
-- Name: project_authorizations_for_migration project_authorizations_for_migration_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.project_authorizations_for_migration
    ADD CONSTRAINT project_authorizations_for_migration_pkey PRIMARY KEY (user_id, project_id);


--
-- Name: project_authorizations project_authorizations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.project_authorizations
    ADD CONSTRAINT project_authorizations_pkey PRIMARY KEY (user_id, project_id, access_level);


--
-- Name: project_settings project_settings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.project_settings
    ADD CONSTRAINT project_settings_pkey PRIMARY KEY (project_id);


--
-- Name: projects projects_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.projects
    ADD CONSTRAINT projects_pkey PRIMARY KEY (id);


--
-- Name: projects projects_star_count_positive; Type: CHECK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.projects
    ADD CONSTRAINT projects_star_count_positive CHECK ((star_count >= 0)) NOT VALID;


--
-- Name: projects_sync_events projects_sync_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.projects_sync_events
    ADD CONSTRAINT projects_sync_events_pkey PRIMARY KEY (id);


--
-- Name: push_rules push_rules_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.push_rules
    ADD CONSTRAINT push_rules_pkey PRIMARY KEY (id);


--
-- Name: reviews reviews_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviews
    ADD CONSTRAINT reviews_pkey PRIMARY KEY (id);


--
-- Name: saml_providers saml_providers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.saml_providers
    ADD CONSTRAINT saml_providers_pkey PRIMARY KEY (id);


--
-- Name: sprints sequence_is_unique_per_iterations_cadence_id; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sprints
    ADD CONSTRAINT sequence_is_unique_per_iterations_cadence_id UNIQUE (iterations_cadence_id, sequence) DEFERRABLE INITIALLY DEFERRED;


--
-- Name: sprints sprints_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sprints
    ADD CONSTRAINT sprints_pkey PRIMARY KEY (id);


--
-- Name: subscription_add_on_purchases subscription_add_on_purchases_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.subscription_add_on_purchases
    ADD CONSTRAINT subscription_add_on_purchases_pkey PRIMARY KEY (id);


--
-- Name: subscription_user_add_on_assignments subscription_user_add_on_assignments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.subscription_user_add_on_assignments
    ADD CONSTRAINT subscription_user_add_on_assignments_pkey PRIMARY KEY (id);


--
-- Name: todos todos_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.todos
    ADD CONSTRAINT todos_pkey PRIMARY KEY (id);


--
-- Name: user_details user_details_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_details
    ADD CONSTRAINT user_details_pkey PRIMARY KEY (user_id);


--
-- Name: user_preferences user_preferences_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_preferences
    ADD CONSTRAINT user_preferences_pkey PRIMARY KEY (id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: work_item_parent_links work_item_parent_links_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.work_item_parent_links
    ADD CONSTRAINT work_item_parent_links_pkey PRIMARY KEY (id);


--
-- Name: work_item_positions work_item_positions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.work_item_positions
    ADD CONSTRAINT work_item_positions_pkey PRIMARY KEY (work_item_id);


--
-- Name: work_item_transitions work_item_transitions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.work_item_transitions
    ADD CONSTRAINT work_item_transitions_pkey PRIMARY KEY (work_item_id);


--
-- Name: add_default_user_assignment_to_user_preferences; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX add_default_user_assignment_to_user_preferences ON public.user_preferences USING btree (default_duo_add_on_assignment_id);


--
-- Name: analytics_index_events_on_created_at_and_author_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX analytics_index_events_on_created_at_and_author_id ON public.events USING btree (created_at, author_id);


--
-- Name: i_compliance_frameworks_on_id_and_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX i_compliance_frameworks_on_id_and_created_at ON public.compliance_management_frameworks USING btree (id, created_at, pipeline_configuration_full_path);


--
-- Name: idx_abuse_reports_user_id_status_and_category; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_abuse_reports_user_id_status_and_category ON public.abuse_reports USING btree (user_id, status, category);


--
-- Name: idx_addon_purchases_on_last_refreshed_at_desc_nulls_last; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_addon_purchases_on_last_refreshed_at_desc_nulls_last ON public.subscription_add_on_purchases USING btree (last_assigned_users_refreshed_at DESC NULLS LAST);


--
-- Name: idx_award_emoji_on_user_emoji_name_awardable_type_awardable_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_award_emoji_on_user_emoji_name_awardable_type_awardable_id ON public.award_emoji USING btree (user_id, name, awardable_type, awardable_id);


--
-- Name: idx_issues_on_project_id_and_created_at_and_id_and_state_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_issues_on_project_id_and_created_at_and_id_and_state_id ON public.issues USING btree (project_id, created_at, id, state_id);


--
-- Name: idx_issues_on_project_id_and_due_date_and_id_and_state_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_issues_on_project_id_and_due_date_and_id_and_state_id ON public.issues USING btree (project_id, due_date, id, state_id) WHERE (due_date IS NOT NULL);


--
-- Name: idx_issues_on_project_id_and_rel_position_and_id_and_state_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_issues_on_project_id_and_rel_position_and_id_and_state_id ON public.issues USING btree (project_id, relative_position, id, state_id);


--
-- Name: idx_issues_on_project_id_and_updated_at_and_id_and_state_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_issues_on_project_id_and_updated_at_and_id_and_state_id ON public.issues USING btree (project_id, updated_at, id, state_id);


--
-- Name: idx_issues_on_project_work_item_type_closed_at_where_closed; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_issues_on_project_work_item_type_closed_at_where_closed ON public.issues USING btree (project_id, work_item_type_id, closed_at) WHERE (state_id = 2);


--
-- Name: idx_issues_root_namespace_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_issues_root_namespace_created_at ON public.issues USING btree ((namespace_traversal_ids[1]), created_at);


--
-- Name: idx_issues_root_namespace_updated_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_issues_root_namespace_updated_at ON public.issues USING btree ((namespace_traversal_ids[1]), updated_at);


--
-- Name: idx_issues_state_id_namespace_traversal_ids; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_issues_state_id_namespace_traversal_ids ON public.issues USING btree (state_id, namespace_traversal_ids);


--
-- Name: idx_member_roles_on_base_access_level; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_member_roles_on_base_access_level ON public.member_roles USING btree (base_access_level);


--
-- Name: idx_members_created_at_user_id_invite_token; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_members_created_at_user_id_invite_token ON public.members USING btree (created_at) WHERE ((invite_token IS NOT NULL) AND (user_id IS NULL));


--
-- Name: idx_members_on_user_and_source_and_source_type_and_member_role; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_members_on_user_and_source_and_source_type_and_member_role ON public.members USING btree (user_id, source_id, source_type, member_role_id);


--
-- Name: idx_merge_requests_on_id_and_merge_jid; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_merge_requests_on_id_and_merge_jid ON public.merge_requests USING btree (id, merge_jid) WHERE ((merge_jid IS NOT NULL) AND (state_id = 4));


--
-- Name: idx_merge_requests_on_merged_state; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_merge_requests_on_merged_state ON public.merge_requests USING btree (id) WHERE (state_id = 3);


--
-- Name: idx_merge_requests_on_source_project_and_branch_state_opened; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_merge_requests_on_source_project_and_branch_state_opened ON public.merge_requests USING btree (source_project_id, source_branch) WHERE (state_id = 1);


--
-- Name: idx_merge_requests_on_unmerged_state_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_merge_requests_on_unmerged_state_id ON public.merge_requests USING btree (id) WHERE (state_id <> 3);


--
-- Name: idx_mrs_on_target_id_and_created_at_and_state_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_mrs_on_target_id_and_created_at_and_state_id ON public.merge_requests USING btree (target_project_id, state_id, created_at, id);


--
-- Name: idx_namespace_settings_on_default_compliance_framework_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_namespace_settings_on_default_compliance_framework_id ON public.namespace_settings USING btree (default_compliance_framework_id);


--
-- Name: idx_namespace_settings_on_last_dormant_members_review_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_namespace_settings_on_last_dormant_members_review_at ON public.namespace_settings USING btree (last_dormant_member_review_at) WHERE (remove_dormant_members = true);


--
-- Name: idx_namespace_settings_on_ns_id_where_ai_audit_storage_enabled; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_namespace_settings_on_ns_id_where_ai_audit_storage_enabled ON public.namespace_settings USING btree (namespace_id) WHERE (ai_audit_events_storage_enabled = true);


--
-- Name: idx_on_compliance_management_frameworks_namespace_id_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_on_compliance_management_frameworks_namespace_id_name ON public.compliance_management_frameworks USING btree (namespace_id, name);


--
-- Name: idx_open_issues_on_project_and_confidential_and_author_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_open_issues_on_project_and_confidential_and_author_and_id ON public.issues USING btree (project_id, confidential, author_id, id) WHERE (state_id = 1);


--
-- Name: idx_personal_access_tokens_on_previous_personal_access_token_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_personal_access_tokens_on_previous_personal_access_token_id ON public.personal_access_tokens USING btree (previous_personal_access_token_id);


--
-- Name: idx_project_repository_check_partial; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_project_repository_check_partial ON public.projects USING btree (repository_storage, created_at) WHERE (last_repository_check_at IS NULL);


--
-- Name: idx_project_settings_on_duo_dep_bump_bc_enabled_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_project_settings_on_duo_dep_bump_bc_enabled_by_id ON public.project_settings USING btree (duo_dependency_bump_breaking_changes_enabled_by_id);


--
-- Name: idx_project_settings_on_pep_bot_access_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_project_settings_on_pep_bot_access_group_id ON public.project_settings USING btree (pipeline_execution_policy_bot_access_group_id);


--
-- Name: idx_project_settings_on_project_id_where_ai_audit_storage; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_project_settings_on_project_id_where_ai_audit_storage ON public.project_settings USING btree (project_id) WHERE (ai_audit_events_storage_enabled = true);


--
-- Name: idx_projects_api_created_at_id_for_archived; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_projects_api_created_at_id_for_archived ON public.projects USING btree (created_at, id) WHERE ((archived = true) AND (pending_delete = false) AND (hidden = false));


--
-- Name: idx_projects_api_created_at_id_for_archived_vis20; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_projects_api_created_at_id_for_archived_vis20 ON public.projects USING btree (created_at, id) WHERE ((archived = true) AND (visibility_level = 20) AND (pending_delete = false) AND (hidden = false));


--
-- Name: idx_projects_api_created_at_id_for_vis10; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_projects_api_created_at_id_for_vis10 ON public.projects USING btree (created_at, id) WHERE ((visibility_level = 10) AND (pending_delete = false) AND (hidden = false));


--
-- Name: idx_projects_id_created_at_disable_overriding_approvers_false; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_projects_id_created_at_disable_overriding_approvers_false ON public.projects USING btree (id, created_at) WHERE ((disable_overriding_approvers_per_merge_request = false) OR (disable_overriding_approvers_per_merge_request IS NULL));


--
-- Name: idx_projects_id_created_at_disable_overriding_approvers_true; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_projects_id_created_at_disable_overriding_approvers_true ON public.projects USING btree (id, created_at) WHERE (disable_overriding_approvers_per_merge_request = true);


--
-- Name: idx_projects_on_repository_storage_last_repository_updated_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_projects_on_repository_storage_last_repository_updated_at ON public.projects USING btree (id, repository_storage, last_repository_updated_at);


--
-- Name: idx_subscription_add_on_purchases_on_started_on_and_expires_on; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_subscription_add_on_purchases_on_started_on_and_expires_on ON public.subscription_add_on_purchases USING btree (started_at, expires_on);


--
-- Name: idx_user_add_on_assignments_on_add_on_purchase_id_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_add_on_assignments_on_add_on_purchase_id_and_id ON public.subscription_user_add_on_assignments USING btree (add_on_purchase_id, id);


--
-- Name: idx_user_details_on_provisioned_by_group_id_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_details_on_provisioned_by_group_id_user_id ON public.user_details USING btree (provisioned_by_group_id, user_id);


--
-- Name: idx_user_preferences_on_knowledge_graph_governing_namespace_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_preferences_on_knowledge_graph_governing_namespace_id ON public.user_preferences USING btree (knowledge_graph_governing_namespace_id);


--
-- Name: index_abuse_reports_on_assignee_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_abuse_reports_on_assignee_id ON public.abuse_reports USING btree (assignee_id);


--
-- Name: index_abuse_reports_on_organization_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_abuse_reports_on_organization_id ON public.abuse_reports USING btree (organization_id);


--
-- Name: index_abuse_reports_on_reporter_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_abuse_reports_on_reporter_id ON public.abuse_reports USING btree (reporter_id);


--
-- Name: index_abuse_reports_on_resolved_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_abuse_reports_on_resolved_by_id ON public.abuse_reports USING btree (resolved_by_id);


--
-- Name: index_abuse_reports_on_status_and_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_abuse_reports_on_status_and_created_at ON public.abuse_reports USING btree (status, created_at);


--
-- Name: index_abuse_reports_on_status_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_abuse_reports_on_status_and_id ON public.abuse_reports USING btree (status, id);


--
-- Name: index_abuse_reports_on_status_and_updated_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_abuse_reports_on_status_and_updated_at ON public.abuse_reports USING btree (status, updated_at);


--
-- Name: index_abuse_reports_on_status_category_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_abuse_reports_on_status_category_and_id ON public.abuse_reports USING btree (status, category, id);


--
-- Name: index_abuse_reports_on_status_reporter_id_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_abuse_reports_on_status_reporter_id_and_id ON public.abuse_reports USING btree (status, reporter_id, id);


--
-- Name: index_add_on_purchases_on_add_on_uid_and_namespace_id_not_null; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_add_on_purchases_on_add_on_uid_and_namespace_id_not_null ON public.subscription_add_on_purchases USING btree (subscription_add_on_uid, namespace_id) NULLS NOT DISTINCT WHERE (subscription_add_on_uid IS NOT NULL);


--
-- Name: index_add_on_purchases_on_organization_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_add_on_purchases_on_organization_id ON public.subscription_add_on_purchases USING btree (organization_id);


--
-- Name: index_award_emoji_on_awardable_type_and_awardable_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_award_emoji_on_awardable_type_and_awardable_id ON public.award_emoji USING btree (awardable_type, awardable_id);


--
-- Name: index_award_emoji_on_namespace_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_award_emoji_on_namespace_id ON public.award_emoji USING btree (namespace_id);


--
-- Name: index_award_emoji_on_organization_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_award_emoji_on_organization_id ON public.award_emoji USING btree (organization_id);


--
-- Name: index_catalog_resources_on_last_30_day_usage_count; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_catalog_resources_on_last_30_day_usage_count ON public.catalog_resources USING btree (last_30_day_usage_count) WHERE (state = 1);


--
-- Name: index_catalog_resources_on_last_30_day_usage_count_updated_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_catalog_resources_on_last_30_day_usage_count_updated_at ON public.catalog_resources USING btree (last_30_day_usage_count_updated_at);


--
-- Name: index_catalog_resources_on_project_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_catalog_resources_on_project_id ON public.catalog_resources USING btree (project_id);


--
-- Name: index_catalog_resources_on_search_vector; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_catalog_resources_on_search_vector ON public.catalog_resources USING gin (search_vector);


--
-- Name: index_catalog_resources_on_state; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_catalog_resources_on_state ON public.catalog_resources USING btree (state);


--
-- Name: index_closed_incidents_on_namespace_id_closed_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_closed_incidents_on_namespace_id_closed_at ON public.issues USING btree (namespace_id, closed_at) WHERE ((state_id = 2) AND (work_item_type_id = 2));


--
-- Name: index_compliance_frameworks_id_where_frameworks_not_null; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_compliance_frameworks_id_where_frameworks_not_null ON public.compliance_management_frameworks USING btree (id) WHERE (pipeline_configuration_full_path IS NOT NULL);


--
-- Name: index_compliance_management_frameworks_on_name_trigram; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_compliance_management_frameworks_on_name_trigram ON public.compliance_management_frameworks USING gin (name public.gin_trgm_ops);


--
-- Name: index_emails_confirmation_token; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_emails_confirmation_token ON public.emails USING btree (confirmation_token);


--
-- Name: index_emails_on_created_at_where_confirmed_at_is_null; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_emails_on_created_at_where_confirmed_at_is_null ON public.emails USING btree (created_at) WHERE (confirmed_at IS NULL);


--
-- Name: index_emails_on_detumbled_email; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_emails_on_detumbled_email ON public.emails USING btree (detumbled_email);


--
-- Name: index_emails_on_email; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_emails_on_email ON public.emails USING btree (email);


--
-- Name: index_emails_on_email_trigram; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_emails_on_email_trigram ON public.emails USING gin (email public.gin_trgm_ops);


--
-- Name: index_emails_on_organization_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_emails_on_organization_id ON public.emails USING btree (organization_id);


--
-- Name: index_emails_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_emails_on_user_id ON public.emails USING btree (user_id);


--
-- Name: index_emails_on_user_id_and_confirmation_token; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_emails_on_user_id_and_confirmation_token ON public.emails USING btree (user_id, confirmation_token);


--
-- Name: index_epics_on_assignee_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_epics_on_assignee_id ON public.epics USING btree (assignee_id);


--
-- Name: index_epics_on_author_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_epics_on_author_id ON public.epics USING btree (author_id);


--
-- Name: index_epics_on_closed_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_epics_on_closed_by_id ON public.epics USING btree (closed_by_id);


--
-- Name: index_epics_on_confidential; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_epics_on_confidential ON public.epics USING btree (confidential);


--
-- Name: index_epics_on_due_date_sourcing_epic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_epics_on_due_date_sourcing_epic_id ON public.epics USING btree (due_date_sourcing_epic_id) WHERE (due_date_sourcing_epic_id IS NOT NULL);


--
-- Name: index_epics_on_due_date_sourcing_milestone_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_epics_on_due_date_sourcing_milestone_id ON public.epics USING btree (due_date_sourcing_milestone_id);


--
-- Name: index_epics_on_end_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_epics_on_end_date ON public.epics USING btree (end_date);


--
-- Name: index_epics_on_group_id_and_external_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_epics_on_group_id_and_external_key ON public.epics USING btree (group_id, external_key) WHERE (external_key IS NOT NULL);


--
-- Name: index_epics_on_group_id_and_iid; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_epics_on_group_id_and_iid ON public.epics USING btree (group_id, iid);


--
-- Name: index_epics_on_group_id_and_iid_varchar_pattern; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_epics_on_group_id_and_iid_varchar_pattern ON public.epics USING btree (group_id, ((iid)::character varying) varchar_pattern_ops);


--
-- Name: index_epics_on_iid; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_epics_on_iid ON public.epics USING btree (iid);


--
-- Name: index_epics_on_last_edited_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_epics_on_last_edited_by_id ON public.epics USING btree (last_edited_by_id);


--
-- Name: index_epics_on_parent_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_epics_on_parent_id ON public.epics USING btree (parent_id);


--
-- Name: index_epics_on_start_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_epics_on_start_date ON public.epics USING btree (start_date);


--
-- Name: index_epics_on_start_date_sourcing_epic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_epics_on_start_date_sourcing_epic_id ON public.epics USING btree (start_date_sourcing_epic_id) WHERE (start_date_sourcing_epic_id IS NOT NULL);


--
-- Name: index_epics_on_start_date_sourcing_milestone_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_epics_on_start_date_sourcing_milestone_id ON public.epics USING btree (start_date_sourcing_milestone_id);


--
-- Name: index_events_author_id_group_id_action_target_type_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_events_author_id_group_id_action_target_type_created_at ON public.events USING btree (author_id, group_id, action, target_type, created_at);


--
-- Name: index_events_author_id_project_id_action_target_type_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_events_author_id_project_id_action_target_type_created_at ON public.events USING btree (author_id, project_id, action, target_type, created_at);


--
-- Name: index_events_for_followed_users; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_events_for_followed_users ON public.events USING btree (author_id, target_type, action, id);


--
-- Name: index_events_for_group_activity; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_events_for_group_activity ON public.events USING btree (group_id, target_type, action, id) WHERE (group_id IS NOT NULL);


--
-- Name: index_events_for_project_activity; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_events_for_project_activity ON public.events USING btree (project_id, target_type, action, id);


--
-- Name: index_events_on_author_id_and_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_events_on_author_id_and_created_at ON public.events USING btree (author_id, created_at);


--
-- Name: index_events_on_author_id_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_events_on_author_id_and_id ON public.events USING btree (author_id, id);


--
-- Name: index_events_on_created_at_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_events_on_created_at_and_id ON public.events USING btree (created_at, id) WHERE (created_at > '2021-08-27 00:00:00+00'::timestamp with time zone);


--
-- Name: index_events_on_group_id_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_events_on_group_id_and_id ON public.events USING btree (group_id, id) WHERE (group_id IS NOT NULL);


--
-- Name: index_events_on_personal_namespace_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_events_on_personal_namespace_id ON public.events USING btree (personal_namespace_id) WHERE (personal_namespace_id IS NOT NULL);


--
-- Name: index_events_on_project_id_and_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_events_on_project_id_and_created_at ON public.events USING btree (project_id, created_at);


--
-- Name: index_events_on_project_id_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_events_on_project_id_and_id ON public.events USING btree (project_id, id);


--
-- Name: index_events_on_target_type_and_target_id_and_fingerprint; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_events_on_target_type_and_target_id_and_fingerprint ON public.events USING btree (target_type, target_id, fingerprint);


--
-- Name: index_groups_on_parent_id_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_groups_on_parent_id_id ON public.namespaces USING btree (parent_id, id) WHERE ((type)::text = 'Group'::text);


--
-- Name: index_groups_on_path_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_groups_on_path_and_id ON public.namespaces USING btree (path, id) WHERE ((type)::text = 'Group'::text);


--
-- Name: index_identities_on_saml_provider_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_identities_on_saml_provider_id ON public.identities USING btree (saml_provider_id) WHERE (saml_provider_id IS NOT NULL);


--
-- Name: index_identities_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_identities_on_user_id ON public.identities USING btree (user_id);


--
-- Name: index_imported_projects_on_import_type_creator_id_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_imported_projects_on_import_type_creator_id_created_at ON public.projects USING btree (import_type, creator_id, created_at) WHERE (import_type IS NOT NULL);


--
-- Name: index_imported_projects_on_import_type_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_imported_projects_on_import_type_id ON public.projects USING btree (import_type, id) WHERE (import_type IS NOT NULL);


--
-- Name: index_issue_assignees_on_namespace_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_issue_assignees_on_namespace_id ON public.issue_assignees USING btree (namespace_id);


--
-- Name: index_issue_assignees_on_user_id_and_issue_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_issue_assignees_on_user_id_and_issue_id ON public.issue_assignees USING btree (user_id, issue_id);


--
-- Name: index_issue_on_project_id_state_id_and_blocking_issues_count; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_issue_on_project_id_state_id_and_blocking_issues_count ON public.issues USING btree (project_id, state_id, blocking_issues_count);


--
-- Name: index_issues_on_author_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_issues_on_author_id ON public.issues USING btree (author_id);


--
-- Name: index_issues_on_closed_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_issues_on_closed_by_id ON public.issues USING btree (closed_by_id);


--
-- Name: index_issues_on_description_trigram_non_latin; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_issues_on_description_trigram_non_latin ON public.issues USING gin (description public.gin_trgm_ops) WHERE (((title)::text !~ similar_escape('[\u0000-\u02FF\u1E00-\u1EFF\u2070-\u218F]*'::text, NULL::text)) OR (description !~ similar_escape('[\u0000-\u02FF\u1E00-\u1EFF\u2070-\u218F]*'::text, NULL::text)));


--
-- Name: index_issues_on_duplicated_to_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_issues_on_duplicated_to_id ON public.issues USING btree (duplicated_to_id) WHERE (duplicated_to_id IS NOT NULL);


--
-- Name: index_issues_on_id_and_weight; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_issues_on_id_and_weight ON public.issues USING btree (id, weight);


--
-- Name: index_issues_on_last_edited_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_issues_on_last_edited_by_id ON public.issues USING btree (last_edited_by_id);


--
-- Name: index_issues_on_milestone_id_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_issues_on_milestone_id_and_id ON public.issues USING btree (milestone_id, id);


--
-- Name: index_issues_on_moved_to_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_issues_on_moved_to_id ON public.issues USING btree (moved_to_id) WHERE (moved_to_id IS NOT NULL);


--
-- Name: index_issues_on_namespace_id_created_at_id_state_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_issues_on_namespace_id_created_at_id_state_id ON public.issues USING btree (namespace_id, created_at, id, state_id);


--
-- Name: index_issues_on_namespace_id_iid_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_issues_on_namespace_id_iid_unique ON public.issues USING btree (namespace_id, iid);


--
-- Name: index_issues_on_namespace_id_relative_position_id_state_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_issues_on_namespace_id_relative_position_id_state_id ON public.issues USING btree (namespace_id, relative_position, id, state_id);


--
-- Name: index_issues_on_namespace_id_updated_at_id_state_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_issues_on_namespace_id_updated_at_id_state_id ON public.issues USING btree (namespace_id, updated_at, id, state_id);


--
-- Name: index_issues_on_project_id_and_iid; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_issues_on_project_id_and_iid ON public.issues USING btree (project_id, iid);


--
-- Name: index_issues_on_project_id_and_upvotes_count; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_issues_on_project_id_and_upvotes_count ON public.issues USING btree (project_id, upvotes_count);


--
-- Name: index_issues_on_project_id_closed_at_desc_state_id_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_issues_on_project_id_closed_at_desc_state_id_and_id ON public.issues USING btree (project_id, closed_at DESC NULLS LAST, state_id, id);


--
-- Name: index_issues_on_promoted_to_epic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_issues_on_promoted_to_epic_id ON public.issues USING btree (promoted_to_epic_id) WHERE (promoted_to_epic_id IS NOT NULL);


--
-- Name: index_issues_on_sprint_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_issues_on_sprint_id ON public.issues USING btree (sprint_id);


--
-- Name: index_issues_on_title_trigram_non_latin; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_issues_on_title_trigram_non_latin ON public.issues USING gin (title public.gin_trgm_ops) WHERE (((title)::text !~ similar_escape('[\u0000-\u02FF\u1E00-\u1EFF\u2070-\u218F]*'::text, NULL::text)) OR (description !~ similar_escape('[\u0000-\u02FF\u1E00-\u1EFF\u2070-\u218F]*'::text, NULL::text)));


--
-- Name: index_issues_on_updated_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_issues_on_updated_at ON public.issues USING btree (updated_at);


--
-- Name: index_issues_on_updated_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_issues_on_updated_by_id ON public.issues USING btree (updated_by_id) WHERE (updated_by_id IS NOT NULL);


--
-- Name: index_issues_on_work_item_type_id_namespace_id_created_at_state; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_issues_on_work_item_type_id_namespace_id_created_at_state ON public.issues USING btree (work_item_type_id, namespace_id, created_at, state_id);


--
-- Name: index_issues_on_work_item_type_id_project_id_created_at_state; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_issues_on_work_item_type_id_project_id_created_at_state ON public.issues USING btree (work_item_type_id, project_id, created_at, state_id);


--
-- Name: index_iterations_cadences_on_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_iterations_cadences_on_group_id ON public.iterations_cadences USING btree (group_id);


--
-- Name: index_loose_foreign_keys_deleted_records_for_partitioned_query; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_loose_foreign_keys_deleted_records_for_partitioned_query ON ONLY public.loose_foreign_keys_deleted_records USING btree (partition, fully_qualified_table_name, consume_after, id) WHERE (status = 1);


--
-- Name: index_member_roles_on_name_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_member_roles_on_name_unique ON public.member_roles USING btree (name) WHERE (namespace_id IS NULL);


--
-- Name: index_member_roles_on_namespace_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_member_roles_on_namespace_id ON public.member_roles USING btree (namespace_id);


--
-- Name: index_member_roles_on_namespace_id_name_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_member_roles_on_namespace_id_name_unique ON public.member_roles USING btree (namespace_id, name) WHERE (namespace_id IS NOT NULL);


--
-- Name: index_member_roles_on_occupies_seat; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_member_roles_on_occupies_seat ON public.member_roles USING btree (occupies_seat);


--
-- Name: index_member_roles_on_organization_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_member_roles_on_organization_id ON public.member_roles USING btree (organization_id);


--
-- Name: index_member_roles_on_permissions; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_member_roles_on_permissions ON public.member_roles USING gin (permissions);


--
-- Name: index_members_on_access_level; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_members_on_access_level ON public.members USING btree (access_level);


--
-- Name: index_members_on_expires_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_members_on_expires_at ON public.members USING btree (expires_at);


--
-- Name: index_members_on_expiring_at_access_level_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_members_on_expiring_at_access_level_id ON public.members USING btree (expires_at, access_level, id) WHERE ((requested_at IS NULL) AND (expiry_notified_at IS NULL));


--
-- Name: index_members_on_invite_email; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_members_on_invite_email ON public.members USING btree (invite_email);


--
-- Name: index_members_on_invite_token; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_members_on_invite_token ON public.members USING btree (invite_token);


--
-- Name: index_members_on_lower_invite_email_with_token; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_members_on_lower_invite_email_with_token ON public.members USING btree (lower((invite_email)::text)) WHERE (invite_token IS NOT NULL);


--
-- Name: index_members_on_member_namespace_id_compound; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_members_on_member_namespace_id_compound ON public.members USING btree (member_namespace_id, type, requested_at, id);


--
-- Name: index_members_on_member_role_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_members_on_member_role_id ON public.members USING btree (member_role_id);


--
-- Name: index_members_on_requested_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_members_on_requested_at ON public.members USING btree (requested_at);


--
-- Name: index_members_on_source_access_level_user_id_member_role_null; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_members_on_source_access_level_user_id_member_role_null ON public.members USING btree (source_id, source_type, access_level, user_id) WHERE (member_role_id IS NULL);


--
-- Name: index_members_on_source_and_type_and_access_level; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_members_on_source_and_type_and_access_level ON public.members USING btree (source_id, source_type, type, access_level);


--
-- Name: index_members_on_source_and_type_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_members_on_source_and_type_and_id ON public.members USING btree (source_id, source_type, type, id) WHERE (invite_token IS NULL);


--
-- Name: index_members_on_source_state_type_access_level_and_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_members_on_source_state_type_access_level_and_user_id ON public.members USING btree (source_id, source_type, state, type, access_level, user_id) WHERE ((requested_at IS NULL) AND (invite_token IS NULL));


--
-- Name: index_members_on_user_id_and_access_level_requested_at_is_null; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_members_on_user_id_and_access_level_requested_at_is_null ON public.members USING btree (user_id, access_level) WHERE (requested_at IS NULL);


--
-- Name: index_members_on_user_id_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_members_on_user_id_created_at ON public.members USING btree (user_id, created_at) WHERE ((ldap = true) AND ((type)::text = 'GroupMember'::text) AND ((source_type)::text = 'Namespace'::text));


--
-- Name: index_merge_request_diffs_by_id_partial; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_request_diffs_by_id_partial ON public.merge_request_diffs USING btree (id) WHERE ((files_count > 0) AND ((NOT stored_externally) OR (stored_externally IS NULL)));


--
-- Name: index_merge_request_diffs_on_external_diff; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_request_diffs_on_external_diff ON public.merge_request_diffs USING btree (external_diff);


--
-- Name: index_merge_request_diffs_on_external_diff_store; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_request_diffs_on_external_diff_store ON public.merge_request_diffs USING btree (external_diff_store);


--
-- Name: index_merge_request_diffs_on_head_commit_sha_bytea; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_request_diffs_on_head_commit_sha_bytea ON public.merge_request_diffs USING btree (head_commit_sha_bytea);


--
-- Name: index_merge_request_diffs_on_merge_request_id_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_request_diffs_on_merge_request_id_and_id ON public.merge_request_diffs USING btree (merge_request_id, id);


--
-- Name: index_merge_request_diffs_on_project_id_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_request_diffs_on_project_id_and_id ON public.merge_request_diffs USING btree (project_id, id);


--
-- Name: index_merge_request_diffs_on_unique_merge_request_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_merge_request_diffs_on_unique_merge_request_id ON public.merge_request_diffs USING btree (merge_request_id) WHERE (diff_type = 2);


--
-- Name: index_merge_requests_for_latest_diffs_with_state_merged; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_requests_for_latest_diffs_with_state_merged ON public.merge_requests USING btree (latest_merge_request_diff_id, target_project_id) WHERE (state_id = 3);


--
-- Name: index_merge_requests_on_assignee_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_requests_on_assignee_id ON public.merge_requests USING btree (assignee_id);


--
-- Name: index_merge_requests_on_author_id_and_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_requests_on_author_id_and_created_at ON public.merge_requests USING btree (author_id, created_at);


--
-- Name: index_merge_requests_on_author_id_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_requests_on_author_id_and_id ON public.merge_requests USING btree (author_id, id);


--
-- Name: index_merge_requests_on_author_id_and_target_project_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_requests_on_author_id_and_target_project_id ON public.merge_requests USING btree (author_id, target_project_id);


--
-- Name: index_merge_requests_on_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_requests_on_created_at ON public.merge_requests USING btree (created_at);


--
-- Name: index_merge_requests_on_head_pipeline_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_requests_on_head_pipeline_id ON public.merge_requests USING btree (head_pipeline_id);


--
-- Name: index_merge_requests_on_latest_merge_request_diff_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_requests_on_latest_merge_request_diff_id ON public.merge_requests USING btree (latest_merge_request_diff_id);


--
-- Name: index_merge_requests_on_merge_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_requests_on_merge_user_id ON public.merge_requests USING btree (merge_user_id) WHERE (merge_user_id IS NOT NULL);


--
-- Name: index_merge_requests_on_milestone_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_requests_on_milestone_id ON public.merge_requests USING btree (milestone_id);


--
-- Name: index_merge_requests_on_source_branch; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_requests_on_source_branch ON public.merge_requests USING btree (source_branch);


--
-- Name: index_merge_requests_on_source_project_id_and_source_branch; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_requests_on_source_project_id_and_source_branch ON public.merge_requests USING btree (source_project_id, source_branch);


--
-- Name: index_merge_requests_on_target_branch; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_requests_on_target_branch ON public.merge_requests USING btree (target_branch);


--
-- Name: index_merge_requests_on_target_project_id_and_created_at_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_requests_on_target_project_id_and_created_at_and_id ON public.merge_requests USING btree (target_project_id, created_at, id);


--
-- Name: index_merge_requests_on_target_project_id_and_iid; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_merge_requests_on_target_project_id_and_iid ON public.merge_requests USING btree (target_project_id, iid);


--
-- Name: index_merge_requests_on_target_project_id_and_merged_commit_sha; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_requests_on_target_project_id_and_merged_commit_sha ON public.merge_requests USING btree (target_project_id, merged_commit_sha);


--
-- Name: index_merge_requests_on_target_project_id_and_source_branch; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_requests_on_target_project_id_and_source_branch ON public.merge_requests USING btree (target_project_id, source_branch);


--
-- Name: index_merge_requests_on_target_project_id_and_squash_commit_sha; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_requests_on_target_project_id_and_squash_commit_sha ON public.merge_requests USING btree (target_project_id, squash_commit_sha);


--
-- Name: index_merge_requests_on_target_project_id_and_target_branch; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_requests_on_target_project_id_and_target_branch ON public.merge_requests USING btree (target_project_id, target_branch) WHERE ((state_id = 1) AND (merge_when_pipeline_succeeds = true));


--
-- Name: index_merge_requests_on_target_project_id_and_updated_at_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_requests_on_target_project_id_and_updated_at_and_id ON public.merge_requests USING btree (target_project_id, updated_at, id);


--
-- Name: index_merge_requests_on_tp_id_and_merge_commit_sha_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_requests_on_tp_id_and_merge_commit_sha_and_id ON public.merge_requests USING btree (target_project_id, merge_commit_sha, id);


--
-- Name: index_merge_requests_on_updated_by_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_merge_requests_on_updated_by_id ON public.merge_requests USING btree (updated_by_id) WHERE (updated_by_id IS NOT NULL);


--
-- Name: index_milestones_on_description_trigram; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_milestones_on_description_trigram ON public.milestones USING gin (description public.gin_trgm_ops);


--
-- Name: index_milestones_on_due_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_milestones_on_due_date ON public.milestones USING btree (due_date);


--
-- Name: index_milestones_on_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_milestones_on_group_id ON public.milestones USING btree (group_id);


--
-- Name: index_milestones_on_project_id_and_iid; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_milestones_on_project_id_and_iid ON public.milestones USING btree (project_id, iid);


--
-- Name: index_milestones_on_title; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_milestones_on_title ON public.milestones USING btree (title);


--
-- Name: index_milestones_on_title_trigram; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_milestones_on_title_trigram ON public.milestones USING gin (title public.gin_trgm_ops);


--
-- Name: index_namespace_details_on_creator_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_namespace_details_on_creator_id ON public.namespace_details USING btree (creator_id);


--
-- Name: index_namespace_details_on_id_and_deletion_scheduled_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_namespace_details_on_id_and_deletion_scheduled_at ON public.namespace_details USING btree (namespace_id, deletion_scheduled_at) WHERE (deletion_scheduled_at IS NOT NULL);


--
-- Name: index_namespace_settings_on_duo_features; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_namespace_settings_on_duo_features ON public.namespace_settings USING btree (duo_features_enabled, lock_duo_features_enabled) INCLUDE (namespace_id) WHERE (duo_features_enabled IS NOT NULL);


--
-- Name: index_namespace_settings_on_namespace_id_where_archived_true; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_namespace_settings_on_namespace_id_where_archived_true ON public.namespace_settings USING btree (namespace_id) WHERE (archived = true);


--
-- Name: index_namespaces_name_parent_id_type; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_namespaces_name_parent_id_type ON public.namespaces USING btree (name, parent_id, type);


--
-- Name: index_namespaces_on_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_namespaces_on_created_at ON public.namespaces USING btree (created_at);


--
-- Name: index_namespaces_on_ldap_sync_last_successful_update_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_namespaces_on_ldap_sync_last_successful_update_at ON public.namespaces USING btree (ldap_sync_last_successful_update_at);


--
-- Name: index_namespaces_on_name_trigram; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_namespaces_on_name_trigram ON public.namespaces USING gin (name public.gin_trgm_ops);


--
-- Name: index_namespaces_on_organization_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_namespaces_on_organization_id ON public.namespaces USING btree (organization_id);


--
-- Name: index_namespaces_on_organization_id_and_id_for_groups; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_namespaces_on_organization_id_and_id_for_groups ON public.namespaces USING btree (organization_id, id) WHERE ((type)::text = 'Group'::text);


--
-- Name: index_namespaces_on_owner_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_namespaces_on_owner_id ON public.namespaces USING btree (owner_id);


--
-- Name: index_namespaces_on_parent_id_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_namespaces_on_parent_id_and_id ON public.namespaces USING btree (parent_id, id);


--
-- Name: index_namespaces_on_path; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_namespaces_on_path ON public.namespaces USING btree (path);


--
-- Name: index_namespaces_on_path_for_top_level_non_projects; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_namespaces_on_path_for_top_level_non_projects ON public.namespaces USING btree (lower((path)::text)) WHERE ((parent_id IS NULL) AND ((type)::text <> 'Project'::text));


--
-- Name: index_namespaces_on_path_trigram; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_namespaces_on_path_trigram ON public.namespaces USING gin (path public.gin_trgm_ops);


--
-- Name: index_namespaces_on_runners_token_encrypted; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_namespaces_on_runners_token_encrypted ON public.namespaces USING btree (runners_token_encrypted);


--
-- Name: index_namespaces_on_traversal_ids; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_namespaces_on_traversal_ids ON public.namespaces USING gin (traversal_ids);


--
-- Name: index_namespaces_on_traversal_ids_for_groups; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_namespaces_on_traversal_ids_for_groups ON public.namespaces USING gin (traversal_ids) WHERE ((type)::text = 'Group'::text);


--
-- Name: index_namespaces_on_traversal_ids_for_groups_btree; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_namespaces_on_traversal_ids_for_groups_btree ON public.namespaces USING btree (traversal_ids) WHERE ((type)::text = 'Group'::text);


--
-- Name: index_namespaces_on_type_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_namespaces_on_type_and_id ON public.namespaces USING btree (type, id);


--
-- Name: index_namespaces_public_groups_name_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_namespaces_public_groups_name_id ON public.namespaces USING btree (name, id) WHERE (((type)::text = 'Group'::text) AND (visibility_level = 20));


--
-- Name: index_namespaces_sync_events_on_namespace_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_namespaces_sync_events_on_namespace_id ON public.namespaces_sync_events USING btree (namespace_id);


--
-- Name: index_non_requested_project_members_on_source_id_and_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_non_requested_project_members_on_source_id_and_type ON public.members USING btree (source_id, source_type) WHERE ((requested_at IS NULL) AND ((type)::text = 'ProjectMember'::text));


--
-- Name: index_notes_for_cherry_picked_merge_requests; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notes_for_cherry_picked_merge_requests ON public.notes USING btree (project_id, commit_id) WHERE ((noteable_type)::text = 'MergeRequest'::text);


--
-- Name: index_notes_on_author_id_and_created_at_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notes_on_author_id_and_created_at_and_id ON public.notes USING btree (author_id, created_at, id);


--
-- Name: index_notes_on_commit_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notes_on_commit_id ON public.notes USING btree (commit_id);


--
-- Name: index_notes_on_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notes_on_created_at ON public.notes USING btree (created_at);


--
-- Name: index_notes_on_discussion_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notes_on_discussion_id ON public.notes USING btree (discussion_id);


--
-- Name: index_notes_on_id_where_internal; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notes_on_id_where_internal ON public.notes USING btree (id) WHERE (internal = true);


--
-- Name: index_notes_on_line_code; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notes_on_line_code ON public.notes USING btree (line_code);


--
-- Name: index_notes_on_namespace_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notes_on_namespace_id ON public.notes USING btree (namespace_id);


--
-- Name: index_notes_on_noteable_id_and_noteable_type_system_author_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notes_on_noteable_id_and_noteable_type_system_author_id ON public.notes USING btree (noteable_id, noteable_type, system) INCLUDE (author_id);


--
-- Name: index_notes_on_noteable_id_noteable_type_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notes_on_noteable_id_noteable_type_and_id ON public.notes USING btree (noteable_id, noteable_type, id);


--
-- Name: index_notes_on_organization_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notes_on_organization_id ON public.notes USING btree (organization_id);


--
-- Name: index_notes_on_project_id_and_id_and_system_false; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notes_on_project_id_and_id_and_system_false ON public.notes USING btree (project_id, id) WHERE (NOT system);


--
-- Name: index_notes_on_project_id_and_noteable_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notes_on_project_id_and_noteable_type ON public.notes USING btree (project_id, noteable_type);


--
-- Name: index_notes_on_review_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notes_on_review_id ON public.notes USING btree (review_id);


--
-- Name: index_notes_on_system_notes_with_mentions; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_notes_on_system_notes_with_mentions ON public.notes USING btree (noteable_id, noteable_type) WHERE ((system = true) AND (note ~~ '%@%'::text));


--
-- Name: index_on_events_to_improve_contribution_analytics_performance; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_on_events_to_improve_contribution_analytics_performance ON public.events USING btree (project_id, target_type, action, created_at, author_id, id);


--
-- Name: index_on_identities_lower_extern_uid_and_provider; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_on_identities_lower_extern_uid_and_provider ON public.identities USING btree (lower((extern_uid)::text), provider);


--
-- Name: index_on_merge_request_diffs_head_commit_sha; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_on_merge_request_diffs_head_commit_sha ON public.merge_request_diffs USING btree (head_commit_sha);


--
-- Name: index_on_merge_requests_for_latest_diffs; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_on_merge_requests_for_latest_diffs ON public.merge_requests USING btree (target_project_id) INCLUDE (id, latest_merge_request_diff_id);


--
-- Name: INDEX index_on_merge_requests_for_latest_diffs; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.index_on_merge_requests_for_latest_diffs IS 'Index used to efficiently obtain the oldest merge request for a commit SHA';


--
-- Name: index_on_namespaces_lower_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_on_namespaces_lower_name ON public.namespaces USING btree (lower((name)::text));


--
-- Name: index_on_namespaces_lower_path; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_on_namespaces_lower_path ON public.namespaces USING btree (lower((path)::text));


--
-- Name: index_on_namespaces_namespaces_by_top_level_namespace; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_on_namespaces_namespaces_by_top_level_namespace ON public.namespaces USING btree ((traversal_ids[1]), type, id);


--
-- Name: index_on_todos_user_project_target_and_state; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_on_todos_user_project_target_and_state ON public.todos USING btree (user_id, project_id, target_type, target_id, id) WHERE ((state)::text = 'pending'::text);


--
-- Name: index_on_users_lower_email; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_on_users_lower_email ON public.users USING btree (lower((email)::text));


--
-- Name: index_on_users_lower_username; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_on_users_lower_username ON public.users USING btree (lower((username)::text));


--
-- Name: index_on_users_name_lower; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_on_users_name_lower ON public.users USING btree (lower((name)::text));


--
-- Name: index_open_issues_on_namespace_id_confidential_author_id_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_open_issues_on_namespace_id_confidential_author_id_id ON public.issues USING btree (namespace_id, confidential, author_id, id) WHERE (state_id = 1);


--
-- Name: index_organizations_on_name_trigram; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_organizations_on_name_trigram ON public.organizations USING gin (name public.gin_trgm_ops);


--
-- Name: index_organizations_on_path_trigram; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_organizations_on_path_trigram ON public.organizations USING gin (path public.gin_trgm_ops);


--
-- Name: index_organizations_on_state; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_organizations_on_state ON public.organizations USING btree (state);


--
-- Name: index_organizations_on_uuid; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_organizations_on_uuid ON public.organizations USING btree (uuid);


--
-- Name: index_p_catalog_resource_sync_events_on_id_where_pending; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_p_catalog_resource_sync_events_on_id_where_pending ON ONLY public.p_catalog_resource_sync_events USING btree (id) WHERE (status = 1);


--
-- Name: index_pat_on_user_id_and_expires_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_pat_on_user_id_and_expires_at ON public.personal_access_tokens USING btree (user_id, expires_at);


--
-- Name: index_pats_on_expiring_at_seven_days_notification_sent_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_pats_on_expiring_at_seven_days_notification_sent_at ON public.personal_access_tokens USING btree (expires_at, id) WHERE ((impersonation = false) AND (revoked = false) AND (seven_days_notification_sent_at IS NULL));


--
-- Name: index_pats_on_expiring_at_sixty_days_notification_sent_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_pats_on_expiring_at_sixty_days_notification_sent_at ON public.personal_access_tokens USING btree (expires_at, id) WHERE ((impersonation = false) AND (revoked = false) AND (sixty_days_notification_sent_at IS NULL));


--
-- Name: index_pats_on_expiring_at_thirty_days_notification_sent_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_pats_on_expiring_at_thirty_days_notification_sent_at ON public.personal_access_tokens USING btree (expires_at, id) WHERE ((impersonation = false) AND (revoked = false) AND (thirty_days_notification_sent_at IS NULL));


--
-- Name: index_pats_on_group_id_and_user_type_and_created_at_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_pats_on_group_id_and_user_type_and_created_at_and_id ON public.personal_access_tokens USING btree (group_id, user_type, created_at, id) WHERE (impersonation = false);


--
-- Name: index_pats_on_group_id_and_user_type_and_expires_at_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_pats_on_group_id_and_user_type_and_expires_at_and_id ON public.personal_access_tokens USING btree (group_id, user_type, expires_at, id) WHERE (impersonation = false);


--
-- Name: index_pats_on_group_id_and_user_type_and_last_used_at_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_pats_on_group_id_and_user_type_and_last_used_at_and_id ON public.personal_access_tokens USING btree (group_id, user_type, last_used_at, id) WHERE (impersonation = false);


--
-- Name: index_personal_access_tokens_on_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_personal_access_tokens_on_group_id ON public.personal_access_tokens USING btree (group_id);


--
-- Name: index_personal_access_tokens_on_id_and_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_personal_access_tokens_on_id_and_created_at ON public.personal_access_tokens USING btree (id, created_at);


--
-- Name: index_personal_access_tokens_on_organization_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_personal_access_tokens_on_organization_id ON public.personal_access_tokens USING btree (organization_id);


--
-- Name: index_personal_access_tokens_on_token_digest; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_personal_access_tokens_on_token_digest ON public.personal_access_tokens USING btree (token_digest);


--
-- Name: index_personal_access_tokens_on_user_id_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_personal_access_tokens_on_user_id_and_id ON public.personal_access_tokens USING btree (user_id, id);


--
-- Name: index_project_authorizations_for_migration_on_project_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_project_authorizations_for_migration_on_project_user ON public.project_authorizations_for_migration USING btree (project_id, user_id);


--
-- Name: index_project_authorizations_on_project_user_access_level; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_project_authorizations_on_project_user_access_level ON public.project_authorizations USING btree (project_id, user_id, access_level);


--
-- Name: index_project_settings_on_legacy_os_license_project_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_project_settings_on_legacy_os_license_project_id ON public.project_settings USING btree (project_id) WHERE (legacy_open_source_license_available = true);


--
-- Name: index_project_settings_on_project_id_partially; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_project_settings_on_project_id_partially ON public.project_settings USING btree (project_id) WHERE (has_vulnerabilities IS TRUE);


--
-- Name: index_project_settings_on_push_rule_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_project_settings_on_push_rule_id ON public.project_settings USING btree (push_rule_id);


--
-- Name: index_projects_api_created_at_id_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_api_created_at_id_desc ON public.projects USING btree (created_at, id DESC);


--
-- Name: index_projects_api_last_activity_at_id_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_api_last_activity_at_id_desc ON public.projects USING btree (last_activity_at, id DESC);


--
-- Name: index_projects_api_name_id_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_api_name_id_desc ON public.projects USING btree (name, id DESC);


--
-- Name: index_projects_api_path_id_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_api_path_id_desc ON public.projects USING btree (path, id DESC);


--
-- Name: index_projects_api_updated_at_id_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_api_updated_at_id_desc ON public.projects USING btree (updated_at, id DESC);


--
-- Name: index_projects_api_vis20_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_api_vis20_created_at ON public.projects USING btree (created_at, id) WHERE (visibility_level = 20);


--
-- Name: index_projects_api_vis20_last_activity_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_api_vis20_last_activity_at ON public.projects USING btree (last_activity_at, id) WHERE (visibility_level = 20);


--
-- Name: index_projects_api_vis20_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_api_vis20_name ON public.projects USING btree (name, id) WHERE (visibility_level = 20);


--
-- Name: index_projects_api_vis20_path; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_api_vis20_path ON public.projects USING btree (path, id) WHERE (visibility_level = 20);


--
-- Name: index_projects_api_vis20_updated_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_api_vis20_updated_at ON public.projects USING btree (updated_at, id) WHERE (visibility_level = 20);


--
-- Name: index_projects_not_aimed_for_deletion; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_not_aimed_for_deletion ON public.projects USING btree (id) WHERE (marked_for_deletion_at IS NULL);


--
-- Name: index_projects_on_creator_id_and_created_at_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_creator_id_and_created_at_and_id ON public.projects USING btree (creator_id, created_at, id);


--
-- Name: index_projects_on_creator_id_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_creator_id_and_id ON public.projects USING btree (creator_id, id);


--
-- Name: index_projects_on_creator_id_import_type_and_created_at_partial; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_creator_id_import_type_and_created_at_partial ON public.projects USING btree (creator_id, import_type, created_at) WHERE (import_type IS NOT NULL);


--
-- Name: index_projects_on_description_trigram; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_description_trigram ON public.projects USING gin (description public.gin_trgm_ops);


--
-- Name: index_projects_on_id_and_archived_and_pending_delete; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_id_and_archived_and_pending_delete ON public.projects USING btree (id) WHERE ((archived = false) AND (pending_delete = false));


--
-- Name: index_projects_on_id_and_marked_for_deletion_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_id_and_marked_for_deletion_at ON public.projects USING btree (id, marked_for_deletion_at) WHERE (marked_for_deletion_at IS NOT NULL);


--
-- Name: index_projects_on_id_partial_for_visibility; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_projects_on_id_partial_for_visibility ON public.projects USING btree (id) WHERE (visibility_level = ANY (ARRAY[10, 20]));


--
-- Name: index_projects_on_id_service_desk_enabled; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_id_service_desk_enabled ON public.projects USING btree (id) WHERE (service_desk_enabled = true);


--
-- Name: index_projects_on_last_activity_at_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_last_activity_at_and_id ON public.projects USING btree (last_activity_at, id);


--
-- Name: index_projects_on_last_repository_check_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_last_repository_check_at ON public.projects USING btree (last_repository_check_at) WHERE (last_repository_check_at IS NOT NULL);


--
-- Name: index_projects_on_last_repository_check_failed; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_last_repository_check_failed ON public.projects USING btree (last_repository_check_failed);


--
-- Name: index_projects_on_last_repository_updated_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_last_repository_updated_at ON public.projects USING btree (last_repository_updated_at);


--
-- Name: index_projects_on_lower_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_lower_name ON public.projects USING btree (lower((name)::text));


--
-- Name: index_projects_on_marked_for_deletion_by_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_marked_for_deletion_by_user_id ON public.projects USING btree (marked_for_deletion_by_user_id) WHERE (marked_for_deletion_by_user_id IS NOT NULL);


--
-- Name: index_projects_on_mirror_creator_id_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_mirror_creator_id_created_at ON public.projects USING btree (creator_id, created_at) WHERE ((mirror = true) AND (mirror_trigger_builds = true));


--
-- Name: index_projects_on_mirror_id_where_mirror_and_trigger_builds; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_mirror_id_where_mirror_and_trigger_builds ON public.projects USING btree (id) WHERE ((mirror = true) AND (mirror_trigger_builds = true));


--
-- Name: index_projects_on_mirror_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_mirror_user_id ON public.projects USING btree (mirror_user_id);


--
-- Name: index_projects_on_name_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_name_and_id ON public.projects USING btree (name, id);


--
-- Name: index_projects_on_name_trigram; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_name_trigram ON public.projects USING gin (name public.gin_trgm_ops);


--
-- Name: index_projects_on_namespace_id_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_namespace_id_and_id ON public.projects USING btree (namespace_id, id);


--
-- Name: index_projects_on_namespace_id_and_repository_size_limit; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_namespace_id_and_repository_size_limit ON public.projects USING btree (namespace_id, repository_size_limit);


--
-- Name: index_projects_on_organization_id_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_organization_id_and_id ON public.projects USING btree (organization_id, id);


--
-- Name: index_projects_on_path_trigram; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_path_trigram ON public.projects USING gin (path public.gin_trgm_ops);


--
-- Name: index_projects_on_pending_delete; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_pending_delete ON public.projects USING btree (pending_delete);


--
-- Name: index_projects_on_pool_repository_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_pool_repository_id ON public.projects USING btree (pool_repository_id) WHERE (pool_repository_id IS NOT NULL);


--
-- Name: index_projects_on_project_namespace_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_projects_on_project_namespace_id ON public.projects USING btree (project_namespace_id);


--
-- Name: index_projects_on_repository_storage; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_repository_storage ON public.projects USING btree (repository_storage);


--
-- Name: index_projects_on_star_count; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_star_count ON public.projects USING btree (star_count);


--
-- Name: index_projects_on_updated_at_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_on_updated_at_and_id ON public.projects USING btree (updated_at, id);


--
-- Name: index_projects_sync_events_on_project_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_projects_sync_events_on_project_id ON public.projects_sync_events USING btree (project_id);


--
-- Name: index_push_rules_on_is_sample; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_push_rules_on_is_sample ON public.push_rules USING btree (is_sample) WHERE is_sample;


--
-- Name: index_push_rules_on_organization_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_push_rules_on_organization_id ON public.push_rules USING btree (organization_id);


--
-- Name: index_push_rules_on_project_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_push_rules_on_project_id ON public.push_rules USING btree (project_id);


--
-- Name: index_requirements_project_id_user_id_id_and_target_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_requirements_project_id_user_id_id_and_target_type ON public.todos USING btree (project_id, user_id, id, target_type);


--
-- Name: index_requirements_user_id_and_target_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_requirements_user_id_and_target_type ON public.todos USING btree (user_id, target_type);


--
-- Name: index_reviews_on_author_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_reviews_on_author_id ON public.reviews USING btree (author_id);


--
-- Name: index_reviews_on_merge_request_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_reviews_on_merge_request_id ON public.reviews USING btree (merge_request_id);


--
-- Name: index_reviews_on_project_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_reviews_on_project_id ON public.reviews USING btree (project_id);


--
-- Name: index_saml_providers_on_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_saml_providers_on_group_id ON public.saml_providers USING btree (group_id);


--
-- Name: index_saml_providers_on_member_role_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_saml_providers_on_member_role_id ON public.saml_providers USING btree (member_role_id);


--
-- Name: index_service_desk_enabled_projects_on_id_creator_id_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_service_desk_enabled_projects_on_id_creator_id_created_at ON public.projects USING btree (id, creator_id, created_at) WHERE (service_desk_enabled = true);


--
-- Name: index_sprints_iterations_cadence_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_sprints_iterations_cadence_id ON public.sprints USING btree (iterations_cadence_id);


--
-- Name: index_sprints_on_description_trigram; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_sprints_on_description_trigram ON public.sprints USING gin (description public.gin_trgm_ops);


--
-- Name: index_sprints_on_due_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_sprints_on_due_date ON public.sprints USING btree (due_date);


--
-- Name: index_sprints_on_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_sprints_on_group_id ON public.sprints USING btree (group_id);


--
-- Name: index_sprints_on_title; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_sprints_on_title ON public.sprints USING btree (title);


--
-- Name: index_sprints_on_title_trigram; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_sprints_on_title_trigram ON public.sprints USING gin (title public.gin_trgm_ops);


--
-- Name: index_subscription_add_on_purchases_on_namespace_id_add_on_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_subscription_add_on_purchases_on_namespace_id_add_on_id ON public.subscription_add_on_purchases USING btree (namespace_id, subscription_add_on_id);


--
-- Name: index_subscription_add_on_purchases_on_subscription_add_on_uid; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_subscription_add_on_purchases_on_subscription_add_on_uid ON public.subscription_add_on_purchases USING btree (subscription_add_on_uid);


--
-- Name: index_subscription_addon_purchases_on_expires_on; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_subscription_addon_purchases_on_expires_on ON public.subscription_add_on_purchases USING btree (expires_on);


--
-- Name: index_subscription_user_add_on_assignments_on_organization_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_subscription_user_add_on_assignments_on_organization_id ON public.subscription_user_add_on_assignments USING btree (organization_id);


--
-- Name: index_subscription_user_add_on_assignments_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_subscription_user_add_on_assignments_on_user_id ON public.subscription_user_add_on_assignments USING btree (user_id);


--
--



--
-- Name: index_todos_on_author_id_and_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_todos_on_author_id_and_created_at ON public.todos USING btree (author_id, created_at);


--
-- Name: index_todos_on_commit_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_todos_on_commit_id ON public.todos USING btree (commit_id);


--
-- Name: index_todos_on_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_todos_on_group_id ON public.todos USING btree (group_id);


--
-- Name: index_todos_on_note_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_todos_on_note_id ON public.todos USING btree (note_id);


--
-- Name: index_todos_on_organization_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_todos_on_organization_id ON public.todos USING btree (organization_id);


--
-- Name: index_todos_on_project_id_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_todos_on_project_id_and_id ON public.todos USING btree (project_id, id);


--
-- Name: index_todos_on_target_type_and_target_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_todos_on_target_type_and_target_id ON public.todos USING btree (target_type, target_id);


--
-- Name: index_todos_on_user_id_and_id_done; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_todos_on_user_id_and_id_done ON public.todos USING btree (user_id, id) WHERE ((state)::text = 'done'::text);


--
-- Name: index_todos_on_user_id_and_id_pending; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_todos_on_user_id_and_id_pending ON public.todos USING btree (user_id, id) WHERE ((state)::text = 'pending'::text);


--
-- Name: index_uniq_projects_on_runners_token; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_uniq_projects_on_runners_token ON public.projects USING btree (runners_token);


--
-- Name: index_uniq_projects_on_runners_token_encrypted; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_uniq_projects_on_runners_token_encrypted ON public.projects USING btree (runners_token_encrypted);


--
-- Name: index_unique_epics_on_issue_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_unique_epics_on_issue_id ON public.epics USING btree (issue_id);


--
-- Name: index_unique_parent_link_id_on_epics; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_unique_parent_link_id_on_epics ON public.epics USING btree (work_item_parent_link_id);


--
-- Name: index_unique_project_authorizations_on_unique_project_user; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_unique_project_authorizations_on_unique_project_user ON public.project_authorizations USING btree (project_id, user_id) WHERE is_unique;


--
-- Name: index_user_details_on_bot_namespace_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_details_on_bot_namespace_id ON public.user_details USING btree (bot_namespace_id);


--
-- Name: index_user_details_on_enterprise_group_id_and_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_details_on_enterprise_group_id_and_user_id ON public.user_details USING btree (enterprise_group_id, user_id);


--
-- Name: index_user_details_on_password_last_changed_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_details_on_password_last_changed_at ON public.user_details USING btree (password_last_changed_at);


--
-- Name: INDEX index_user_details_on_password_last_changed_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.index_user_details_on_password_last_changed_at IS 'JiHu-specific index';


--
-- Name: index_user_details_on_phone; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_user_details_on_phone ON public.user_details USING btree (phone) WHERE (phone IS NOT NULL);


--
-- Name: INDEX index_user_details_on_phone; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX public.index_user_details_on_phone IS 'JiHu-specific index';


--
-- Name: index_user_details_on_provisioned_by_project_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_details_on_provisioned_by_project_id ON public.user_details USING btree (provisioned_by_project_id);


--
-- Name: index_user_details_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_user_details_on_user_id ON public.user_details USING btree (user_id);


--
-- Name: index_user_preferences_on_duo_default_namespace_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_preferences_on_duo_default_namespace_id ON public.user_preferences USING btree (duo_default_namespace_id);


--
-- Name: index_user_preferences_on_gitpod_enabled; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_preferences_on_gitpod_enabled ON public.user_preferences USING btree (gitpod_enabled);


--
-- Name: index_user_preferences_on_policy_advanced_editor; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_preferences_on_policy_advanced_editor ON public.user_preferences USING btree (policy_advanced_editor) WHERE (policy_advanced_editor = true);


--
-- Name: index_user_preferences_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_user_preferences_on_user_id ON public.user_preferences USING btree (user_id);


--
-- Name: index_user_preferences_on_user_id_emoji_autocomplete_disabled; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_user_preferences_on_user_id_emoji_autocomplete_disabled ON public.user_preferences USING btree (user_id) WHERE (emoji_autocomplete_enabled = false);


--
-- Name: index_users_for_active_billable_users; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_for_active_billable_users ON public.users USING btree (id) WHERE (((state)::text = 'active'::text) AND (user_type = ANY (ARRAY[0, 6, 4, 13])) AND (user_type = ANY (ARRAY[0, 4, 5])));


--
-- Name: index_users_for_auditors; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_for_auditors ON public.users USING btree (id) WHERE (auditor IS TRUE);


--
-- Name: index_users_on_admin; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_admin ON public.users USING btree (admin);


--
-- Name: index_users_on_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_created_at ON public.users USING btree (created_at);


--
-- Name: index_users_on_email; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_users_on_email ON public.users USING btree (email);


--
-- Name: index_users_on_email_domain_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_email_domain_and_id ON public.users USING btree (lower(split_part((email)::text, '@'::text, 2)), id);


--
-- Name: index_users_on_email_trigram; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_email_trigram ON public.users USING gin (email public.gin_trgm_ops);


--
-- Name: index_users_on_feed_token; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_feed_token ON public.users USING btree (feed_token);


--
-- Name: index_users_on_group_view; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_group_view ON public.users USING btree (group_view);


--
-- Name: index_users_on_id_and_last_activity_on_for_active_human_service; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_id_and_last_activity_on_for_active_human_service ON public.users USING btree (id, last_activity_on) WHERE (((state)::text = 'active'::text) AND (user_type = ANY (ARRAY[0, 4])));


--
-- Name: index_users_on_incoming_email_token; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_incoming_email_token ON public.users USING btree (incoming_email_token);


--
-- Name: index_users_on_managing_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_managing_group_id ON public.users USING btree (managing_group_id);


--
-- Name: index_users_on_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_name ON public.users USING btree (name);


--
-- Name: index_users_on_name_trigram; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_name_trigram ON public.users USING gin (name public.gin_trgm_ops);


--
-- Name: index_users_on_organization_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_organization_id ON public.users USING btree (organization_id);


--
-- Name: index_users_on_organization_id_and_confirmation_token; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_users_on_organization_id_and_confirmation_token ON public.users USING btree (organization_id, confirmation_token);


--
-- Name: index_users_on_organization_id_and_reset_password_token; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_users_on_organization_id_and_reset_password_token ON public.users USING btree (organization_id, reset_password_token);


--
-- Name: index_users_on_organization_id_and_unlock_token; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_users_on_organization_id_and_unlock_token ON public.users USING btree (organization_id, unlock_token);


--
-- Name: index_users_on_public_email_excluding_null_and_empty; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_public_email_excluding_null_and_empty ON public.users USING btree (public_email) WHERE (((public_email)::text <> ''::text) AND (public_email IS NOT NULL));


--
-- Name: index_users_on_public_email_trigram; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_public_email_trigram ON public.users USING gin (public_email public.gin_trgm_ops);


--
-- Name: index_users_on_state_and_user_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_state_and_user_type ON public.users USING btree (state, user_type);


--
-- Name: index_users_on_static_object_token; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_users_on_static_object_token ON public.users USING btree (static_object_token);


--
-- Name: index_users_on_unconfirmed_created_at_active_type_sign_in_count; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_unconfirmed_created_at_active_type_sign_in_count ON public.users USING btree (created_at, id) WHERE ((confirmed_at IS NULL) AND ((state)::text = 'active'::text) AND (user_type = 0) AND (sign_in_count = 0));


--
-- Name: index_users_on_unconfirmed_email; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_unconfirmed_email ON public.users USING btree (unconfirmed_email) WHERE (unconfirmed_email IS NOT NULL);


--
-- Name: index_users_on_updated_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_updated_at ON public.users USING btree (updated_at);


--
-- Name: index_users_on_user_type_and_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_user_type_and_id ON public.users USING btree (user_type, id);


--
-- Name: index_users_on_username; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_username ON public.users USING btree (username);


--
-- Name: index_users_on_username_trigram; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_username_trigram ON public.users USING gin (username public.gin_trgm_ops);


--
-- Name: index_wi_positions_on_positioning_ns_id_and_relative_position; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_wi_positions_on_positioning_ns_id_and_relative_position ON public.work_item_positions USING btree (relative_positioning_namespace_id, relative_position);


--
-- Name: index_work_item_parent_links_on_namespace_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_work_item_parent_links_on_namespace_id ON public.work_item_parent_links USING btree (namespace_id);


--
-- Name: index_work_item_parent_links_on_work_item_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX index_work_item_parent_links_on_work_item_id ON public.work_item_parent_links USING btree (work_item_id);


--
-- Name: index_work_item_parent_links_on_work_item_parent_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_work_item_parent_links_on_work_item_parent_id ON public.work_item_parent_links USING btree (work_item_parent_id);


--
-- Name: index_work_item_positions_on_namespace_id_and_relative_position; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_work_item_positions_on_namespace_id_and_relative_position ON public.work_item_positions USING btree (namespace_id, relative_position);


--
-- Name: index_work_item_transitions_on_duplicated_to_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_work_item_transitions_on_duplicated_to_id ON public.work_item_transitions USING btree (duplicated_to_id) WHERE (duplicated_to_id IS NOT NULL);


--
-- Name: index_work_item_transitions_on_moved_to_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_work_item_transitions_on_moved_to_id ON public.work_item_transitions USING btree (moved_to_id) WHERE (moved_to_id IS NOT NULL);


--
-- Name: index_work_item_transitions_on_namespace_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_work_item_transitions_on_namespace_id ON public.work_item_transitions USING btree (namespace_id);


--
-- Name: index_work_item_transitions_on_promoted_to_epic_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_work_item_transitions_on_promoted_to_epic_id ON public.work_item_transitions USING btree (promoted_to_epic_id) WHERE (promoted_to_epic_id IS NOT NULL);


--
-- Name: temp_index_on_users_where_dark_theme; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX temp_index_on_users_where_dark_theme ON public.users USING btree (id) WHERE (theme_id = 11);


--
-- Name: tmp_idx_members_on_access_level_source_type_requested_at_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tmp_idx_members_on_access_level_source_type_requested_at_id ON public.members USING btree (id) WHERE ((access_level = 5) AND ((source_type)::text = 'Namespace'::text) AND (requested_at IS NULL));


--
-- Name: tmp_index_namespace_settings_on_experiment_features_enabled; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tmp_index_namespace_settings_on_experiment_features_enabled ON public.namespace_settings USING btree (namespace_id) WHERE (experiment_features_enabled IS TRUE);


--
-- Name: tmp_index_notes_on_id_with_namespace_and_project; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tmp_index_notes_on_id_with_namespace_and_project ON public.notes USING btree (id) WHERE ((namespace_id IS NOT NULL) AND (project_id IS NOT NULL));


--
-- Name: tmp_index_users_on_external_where_external_is_null; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tmp_index_users_on_external_where_external_is_null ON public.users USING btree (external) WHERE (external IS NULL);


--
-- Name: uniq_idx_user_add_on_assignments_on_add_on_purchase_and_user; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uniq_idx_user_add_on_assignments_on_add_on_purchase_and_user ON public.subscription_user_add_on_assignments USING btree (add_on_purchase_id, user_id);


--
-- Name: unique_index_for_project_pages_unique_domain; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX unique_index_for_project_pages_unique_domain ON public.project_settings USING btree (pages_unique_domain) WHERE (pages_unique_domain IS NOT NULL);


--
-- Name: unique_organizations_on_path_case_insensitive; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX unique_organizations_on_path_case_insensitive ON public.organizations USING btree (lower(path));


--
-- Name: unique_projects_on_name_namespace_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX unique_projects_on_name_namespace_id ON public.projects USING btree (name, namespace_id);


--
-- Name: issues issues_loose_fk_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER issues_loose_fk_trigger AFTER DELETE ON public.issues REFERENCING OLD TABLE AS old_table FOR EACH STATEMENT EXECUTE FUNCTION public.insert_into_loose_foreign_keys_deleted_records();


--
-- Name: merge_request_diffs merge_request_diffs_loose_fk_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER merge_request_diffs_loose_fk_trigger AFTER DELETE ON public.merge_request_diffs REFERENCING OLD TABLE AS old_table FOR EACH STATEMENT EXECUTE FUNCTION public.insert_into_loose_foreign_keys_deleted_records();


--
-- Name: merge_request_diffs merge_request_diffs_sync_bytea_sha_on_insert; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER merge_request_diffs_sync_bytea_sha_on_insert BEFORE INSERT ON public.merge_request_diffs FOR EACH ROW EXECUTE FUNCTION public.merge_request_diffs_sync_bytea_sha_on_insert();


--
-- Name: merge_request_diffs merge_request_diffs_sync_bytea_sha_on_update; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER merge_request_diffs_sync_bytea_sha_on_update BEFORE UPDATE OF base_commit_sha, base_commit_sha_bytea, start_commit_sha, start_commit_sha_bytea, head_commit_sha, head_commit_sha_bytea ON public.merge_request_diffs FOR EACH ROW EXECUTE FUNCTION public.merge_request_diffs_sync_bytea_sha_on_update();


--
-- Name: merge_requests merge_requests_loose_fk_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER merge_requests_loose_fk_trigger AFTER DELETE ON public.merge_requests REFERENCING OLD TABLE AS old_table FOR EACH STATEMENT EXECUTE FUNCTION public.insert_into_loose_foreign_keys_deleted_records();


--
-- Name: namespaces namespaces_loose_fk_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER namespaces_loose_fk_trigger AFTER DELETE ON public.namespaces REFERENCING OLD TABLE AS old_table FOR EACH STATEMENT EXECUTE FUNCTION public.insert_into_loose_foreign_keys_deleted_records();


--
-- Name: notes notes_loose_fk_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER notes_loose_fk_trigger AFTER DELETE ON public.notes REFERENCING OLD TABLE AS old_table FOR EACH STATEMENT EXECUTE FUNCTION public.insert_into_loose_foreign_keys_deleted_records();


--
-- Name: organizations organizations_loose_fk_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER organizations_loose_fk_trigger AFTER DELETE ON public.organizations REFERENCING OLD TABLE AS old_table FOR EACH STATEMENT EXECUTE FUNCTION public.insert_into_loose_foreign_keys_deleted_records();


--
-- Name: organizations prevent_delete_of_default_organization_before_destroy; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER prevent_delete_of_default_organization_before_destroy BEFORE DELETE ON public.organizations FOR EACH ROW EXECUTE FUNCTION public.prevent_delete_of_default_organization();


--
-- Name: projects projects_loose_fk_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER projects_loose_fk_trigger AFTER DELETE ON public.projects REFERENCING OLD TABLE AS old_table FOR EACH STATEMENT EXECUTE FUNCTION public.insert_into_loose_foreign_keys_deleted_records();


--
-- Name: push_rules push_rules_loose_fk_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER push_rules_loose_fk_trigger AFTER DELETE ON public.push_rules REFERENCING OLD TABLE AS old_table FOR EACH STATEMENT EXECUTE FUNCTION public.insert_into_loose_foreign_keys_deleted_records();


--
-- Name: project_authorizations sync_project_authorizations_to_migration; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER sync_project_authorizations_to_migration AFTER INSERT OR DELETE OR UPDATE ON public.project_authorizations FOR EACH ROW EXECUTE FUNCTION public.sync_project_authorizations_to_migration_table();


--
-- Name: work_item_parent_links trigger_25c44c30884f; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trigger_25c44c30884f BEFORE INSERT OR UPDATE ON public.work_item_parent_links FOR EACH ROW EXECUTE FUNCTION public.trigger_25c44c30884f();


--
-- Name: subscription_user_add_on_assignments trigger_740afa9807b8; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trigger_740afa9807b8 BEFORE INSERT OR UPDATE ON public.subscription_user_add_on_assignments FOR EACH ROW EXECUTE FUNCTION public.trigger_740afa9807b8();


--
-- Name: abuse_reports trigger_7e2eed79e46e; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trigger_7e2eed79e46e BEFORE INSERT OR UPDATE ON public.abuse_reports FOR EACH ROW EXECUTE FUNCTION public.trigger_7e2eed79e46e();


--
-- Name: notes trigger_817aa51bc4f2; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trigger_817aa51bc4f2 BEFORE UPDATE ON public.notes FOR EACH ROW WHEN (((new.namespace_id IS NOT NULL) AND (new.project_id IS NOT NULL))) EXECUTE FUNCTION public.repair_dual_sharding_key_on_notes();


--
-- Name: issue_assignees trigger_97e9245e767d; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trigger_97e9245e767d BEFORE INSERT OR UPDATE ON public.issue_assignees FOR EACH ROW EXECUTE FUNCTION public.trigger_97e9245e767d();


--
-- Name: projects trigger_catalog_resource_sync_event_on_project_update; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trigger_catalog_resource_sync_event_on_project_update AFTER UPDATE ON public.projects FOR EACH ROW WHEN ((((old.name)::text IS DISTINCT FROM (new.name)::text) OR (old.description IS DISTINCT FROM new.description) OR (old.visibility_level IS DISTINCT FROM new.visibility_level))) EXECUTE FUNCTION public.insert_catalog_resource_sync_event();


--
-- Name: projects trigger_delete_project_namespace_on_project_delete; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trigger_delete_project_namespace_on_project_delete AFTER DELETE ON public.projects FOR EACH ROW WHEN ((old.project_namespace_id IS NOT NULL)) EXECUTE FUNCTION public.delete_associated_project_namespace();


--
-- Name: abuse_reports trigger_f7464057d53e; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trigger_f7464057d53e BEFORE INSERT OR UPDATE ON public.abuse_reports FOR EACH ROW EXECUTE FUNCTION public.trigger_f7464057d53e();


--
-- Name: namespaces trigger_namespaces_traversal_ids_on_update; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trigger_namespaces_traversal_ids_on_update AFTER UPDATE ON public.namespaces FOR EACH ROW WHEN ((old.traversal_ids IS DISTINCT FROM new.traversal_ids)) EXECUTE FUNCTION public.insert_namespaces_sync_event();


--
-- Name: projects trigger_projects_parent_id_on_insert; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trigger_projects_parent_id_on_insert AFTER INSERT ON public.projects FOR EACH ROW EXECUTE FUNCTION public.insert_projects_sync_event();


--
-- Name: projects trigger_projects_parent_id_on_update; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trigger_projects_parent_id_on_update AFTER UPDATE ON public.projects FOR EACH ROW WHEN ((old.namespace_id IS DISTINCT FROM new.namespace_id)) EXECUTE FUNCTION public.insert_projects_sync_event();


--
-- Name: issues trigger_sync_work_item_positions_from_issues; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trigger_sync_work_item_positions_from_issues AFTER INSERT OR UPDATE OF relative_position, namespace_id ON public.issues FOR EACH ROW EXECUTE FUNCTION public.sync_work_item_positions_from_issues();


--
-- Name: issues trigger_sync_work_item_transitions_from_issues; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trigger_sync_work_item_transitions_from_issues AFTER INSERT OR UPDATE OF moved_to_id, duplicated_to_id, promoted_to_epic_id, namespace_id ON public.issues FOR EACH ROW EXECUTE FUNCTION public.sync_work_item_transitions_from_issues();


--
-- Name: todos trigger_todos_sharding_key; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trigger_todos_sharding_key BEFORE INSERT OR UPDATE ON public.todos FOR EACH ROW EXECUTE FUNCTION public.todos_sharding_key();


--
-- Name: projects trigger_update_details_on_project_insert; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trigger_update_details_on_project_insert AFTER INSERT ON public.projects FOR EACH ROW EXECUTE FUNCTION public.update_namespace_details_from_projects();


--
-- Name: projects trigger_update_details_on_project_update; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trigger_update_details_on_project_update AFTER UPDATE ON public.projects FOR EACH ROW WHEN (((old.description IS DISTINCT FROM new.description) OR (old.description_html IS DISTINCT FROM new.description_html) OR (old.cached_markdown_version IS DISTINCT FROM new.cached_markdown_version))) EXECUTE FUNCTION public.update_namespace_details_from_projects();


--
-- Name: users users_loose_fk_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER users_loose_fk_trigger AFTER DELETE ON public.users REFERENCING OLD TABLE AS old_table FOR EACH STATEMENT EXECUTE FUNCTION public.insert_into_loose_foreign_keys_deleted_records();


--
-- Name: issues validate_work_item_type_on_insert_or_update_issues; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER validate_work_item_type_on_insert_or_update_issues BEFORE INSERT OR UPDATE OF work_item_type_id ON public.issues FOR EACH ROW EXECUTE FUNCTION public.validate_work_item_type_id_is_valid();


--
-- Name: epics fk_013c9f36ca; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.epics
    ADD CONSTRAINT fk_013c9f36ca FOREIGN KEY (due_date_sourcing_epic_id) REFERENCES public.epics(id) ON DELETE SET NULL;


--
-- Name: work_item_transitions fk_01ba2355cd; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.work_item_transitions
    ADD CONSTRAINT fk_01ba2355cd FOREIGN KEY (promoted_to_epic_id) REFERENCES public.epics(id) ON DELETE SET NULL;


--
-- Name: issues fk_05f1e72feb; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issues
    ADD CONSTRAINT fk_05f1e72feb FOREIGN KEY (author_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: merge_requests fk_06067f5644; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merge_requests
    ADD CONSTRAINT fk_06067f5644 FOREIGN KEY (latest_merge_request_diff_id) REFERENCES public.merge_request_diffs(id) ON DELETE SET NULL;


--
-- Name: work_item_positions fk_0626c1ce18; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.work_item_positions
    ADD CONSTRAINT fk_0626c1ce18 FOREIGN KEY (namespace_id) REFERENCES public.namespaces(id) ON DELETE CASCADE;


--
-- Name: subscription_user_add_on_assignments fk_0d89020c49; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.subscription_user_add_on_assignments
    ADD CONSTRAINT fk_0d89020c49 FOREIGN KEY (add_on_purchase_id) REFERENCES public.subscription_add_on_purchases(id) ON DELETE CASCADE;


--
-- Name: epics fk_1fbed67632; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.epics
    ADD CONSTRAINT fk_1fbed67632 FOREIGN KEY (start_date_sourcing_milestone_id) REFERENCES public.milestones(id) ON DELETE SET NULL;


--
-- Name: namespace_settings fk_20cf0eb2f9; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.namespace_settings
    ADD CONSTRAINT fk_20cf0eb2f9 FOREIGN KEY (default_compliance_framework_id) REFERENCES public.compliance_management_frameworks(id) ON DELETE SET NULL;


--
-- Name: work_item_transitions fk_247358ddff; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.work_item_transitions
    ADD CONSTRAINT fk_247358ddff FOREIGN KEY (work_item_id) REFERENCES public.issues(id) ON DELETE CASCADE;


--
-- Name: user_details fk_27ac767d6a; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_details
    ADD CONSTRAINT fk_27ac767d6a FOREIGN KEY (bot_namespace_id) REFERENCES public.namespaces(id) ON DELETE SET NULL;


--
-- Name: notes fk_2e82291620; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notes
    ADD CONSTRAINT fk_2e82291620 FOREIGN KEY (review_id) REFERENCES public.reviews(id) ON DELETE SET NULL;


--
-- Name: members fk_2f85abf8f1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.members
    ADD CONSTRAINT fk_2f85abf8f1 FOREIGN KEY (member_namespace_id) REFERENCES public.namespaces(id) ON DELETE CASCADE;


--
-- Name: namespaces fk_34fceca87c; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.namespaces
    ADD CONSTRAINT fk_34fceca87c FOREIGN KEY (organization_id) REFERENCES public.organizations(id) ON DELETE CASCADE;


--
-- Name: saml_providers fk_351dde3a84; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.saml_providers
    ADD CONSTRAINT fk_351dde3a84 FOREIGN KEY (member_role_id) REFERENCES public.member_roles(id) ON DELETE SET NULL;


--
-- Name: epics fk_3654b61b03; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.epics
    ADD CONSTRAINT fk_3654b61b03 FOREIGN KEY (author_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: sprints fk_365d1db505; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sprints
    ADD CONSTRAINT fk_365d1db505 FOREIGN KEY (iterations_cadence_id) REFERENCES public.iterations_cadences(id) ON DELETE CASCADE;


--
-- Name: issues fk_3b8c72ea56; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issues
    ADD CONSTRAINT fk_3b8c72ea56 FOREIGN KEY (sprint_id) REFERENCES public.sprints(id) ON DELETE SET NULL;


--
-- Name: epics fk_3c1fd1cccc; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.epics
    ADD CONSTRAINT fk_3c1fd1cccc FOREIGN KEY (due_date_sourcing_milestone_id) REFERENCES public.milestones(id) ON DELETE SET NULL;


--
-- Name: abuse_reports fk_3fe6467b93; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.abuse_reports
    ADD CONSTRAINT fk_3fe6467b93 FOREIGN KEY (assignee_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: todos fk_45054f9c45; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.todos
    ADD CONSTRAINT fk_45054f9c45 FOREIGN KEY (project_id) REFERENCES public.projects(id) ON DELETE CASCADE;


--
-- Name: issue_assignees fk_4b97267a3e; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_assignees
    ADD CONSTRAINT fk_4b97267a3e FOREIGN KEY (namespace_id) REFERENCES public.namespaces(id) ON DELETE CASCADE;


--
-- Name: identities fk_5373344100; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.identities
    ADD CONSTRAINT fk_5373344100 FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_preferences fk_561e98d37c; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_preferences
    ADD CONSTRAINT fk_561e98d37c FOREIGN KEY (default_duo_add_on_assignment_id) REFERENCES public.subscription_user_add_on_assignments(id) ON DELETE SET NULL;


--
-- Name: merge_request_diffs fk_56ac6fc9c0; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merge_request_diffs
    ADD CONSTRAINT fk_56ac6fc9c0 FOREIGN KEY (project_id) REFERENCES public.projects(id) ON DELETE CASCADE;


--
-- Name: award_emoji fk_5e03b44d0b; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.award_emoji
    ADD CONSTRAINT fk_5e03b44d0b FOREIGN KEY (organization_id) REFERENCES public.organizations(id) ON DELETE CASCADE;


--
-- Name: issue_assignees fk_5e0c8d9154; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_assignees
    ADD CONSTRAINT fk_5e0c8d9154 FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: emails fk_5f7164ec3d; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.emails
    ADD CONSTRAINT fk_5f7164ec3d FOREIGN KEY (organization_id) REFERENCES public.organizations(id) ON DELETE CASCADE NOT VALID;


--
-- Name: merge_requests fk_6149611a04; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merge_requests
    ADD CONSTRAINT fk_6149611a04 FOREIGN KEY (assignee_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: user_preferences fk_61f4fd80d1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_preferences
    ADD CONSTRAINT fk_61f4fd80d1 FOREIGN KEY (duo_default_namespace_id) REFERENCES public.namespaces(id) ON DELETE SET NULL;


--
-- Name: merge_requests fk_641731faff; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merge_requests
    ADD CONSTRAINT fk_641731faff FOREIGN KEY (updated_by_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: merge_requests fk_6a5165a692; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merge_requests
    ADD CONSTRAINT fk_6a5165a692 FOREIGN KEY (milestone_id) REFERENCES public.milestones(id) ON DELETE SET NULL;


--
-- Name: projects fk_6ca23af0a3; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.projects
    ADD CONSTRAINT fk_6ca23af0a3 FOREIGN KEY (project_namespace_id) REFERENCES public.namespaces(id) ON DELETE CASCADE;


--
-- Name: subscription_user_add_on_assignments fk_724c2df9a8; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.subscription_user_add_on_assignments
    ADD CONSTRAINT fk_724c2df9a8 FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: epics fk_765e132668; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.epics
    ADD CONSTRAINT fk_765e132668 FOREIGN KEY (work_item_parent_link_id) REFERENCES public.work_item_parent_links(id) ON DELETE SET NULL;


--
-- Name: notes fk_76db6d50c6; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notes
    ADD CONSTRAINT fk_76db6d50c6 FOREIGN KEY (namespace_id) REFERENCES public.namespaces(id) ON DELETE CASCADE;


--
-- Name: todos fk_78558e5d74; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.todos
    ADD CONSTRAINT fk_78558e5d74 FOREIGN KEY (organization_id) REFERENCES public.organizations(id) ON DELETE CASCADE;


--
-- Name: personal_access_tokens fk_7cea2c7262; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.personal_access_tokens
    ADD CONSTRAINT fk_7cea2c7262 FOREIGN KEY (group_id) REFERENCES public.namespaces(id) ON DELETE SET NULL;


--
-- Name: namespaces fk_7f813d8c90; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.namespaces
    ADD CONSTRAINT fk_7f813d8c90 FOREIGN KEY (parent_id) REFERENCES public.namespaces(id) ON DELETE RESTRICT NOT VALID;


--
-- Name: sprints fk_80aa8a1f95; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sprints
    ADD CONSTRAINT fk_80aa8a1f95 FOREIGN KEY (group_id) REFERENCES public.namespaces(id) ON DELETE CASCADE;


--
-- Name: project_settings fk_8264eab4ae; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.project_settings
    ADD CONSTRAINT fk_8264eab4ae FOREIGN KEY (pipeline_execution_policy_bot_access_group_id) REFERENCES public.namespaces(id) ON DELETE SET NULL;


--
-- Name: push_rules fk_83b29894de; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.push_rules
    ADD CONSTRAINT fk_83b29894de FOREIGN KEY (project_id) REFERENCES public.projects(id) ON DELETE CASCADE;


--
-- Name: merge_request_diffs fk_8483f3258f; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merge_request_diffs
    ADD CONSTRAINT fk_8483f3258f FOREIGN KEY (merge_request_id) REFERENCES public.merge_requests(id) ON DELETE CASCADE;


--
-- Name: issues fk_899c8f3231; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issues
    ADD CONSTRAINT fk_899c8f3231 FOREIGN KEY (project_id) REFERENCES public.projects(id) ON DELETE CASCADE;


--
-- Name: user_preferences fk_8f5100f91c; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_preferences
    ADD CONSTRAINT fk_8f5100f91c FOREIGN KEY (knowledge_graph_governing_namespace_id) REFERENCES public.namespaces(id) ON DELETE SET NULL;


--
-- Name: todos fk_91d1f47b13; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.todos
    ADD CONSTRAINT fk_91d1f47b13 FOREIGN KEY (note_id) REFERENCES public.notes(id) ON DELETE CASCADE;


--
-- Name: milestones fk_95650a40d4; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.milestones
    ADD CONSTRAINT fk_95650a40d4 FOREIGN KEY (group_id) REFERENCES public.namespaces(id) ON DELETE CASCADE;


--
-- Name: issues fk_96b1dd429c; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issues
    ADD CONSTRAINT fk_96b1dd429c FOREIGN KEY (milestone_id) REFERENCES public.milestones(id) ON DELETE SET NULL;


--
-- Name: notes fk_99e097b079; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notes
    ADD CONSTRAINT fk_99e097b079 FOREIGN KEY (project_id) REFERENCES public.projects(id) ON DELETE CASCADE;


--
-- Name: projects fk_9aee26923d; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.projects
    ADD CONSTRAINT fk_9aee26923d FOREIGN KEY (organization_id) REFERENCES public.organizations(id) ON DELETE CASCADE;


--
-- Name: work_item_transitions fk_9ba5313b4f; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.work_item_transitions
    ADD CONSTRAINT fk_9ba5313b4f FOREIGN KEY (duplicated_to_id) REFERENCES public.issues(id) ON DELETE SET NULL;


--
-- Name: milestones fk_9bd0a0c791; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.milestones
    ADD CONSTRAINT fk_9bd0a0c791 FOREIGN KEY (project_id) REFERENCES public.projects(id) ON DELETE CASCADE;


--
-- Name: work_item_parent_links fk_9be5ef5f80; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.work_item_parent_links
    ADD CONSTRAINT fk_9be5ef5f80 FOREIGN KEY (namespace_id) REFERENCES public.namespaces(id) ON DELETE CASCADE;


--
-- Name: issues fk_9c4516d665; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issues
    ADD CONSTRAINT fk_9c4516d665 FOREIGN KEY (duplicated_to_id) REFERENCES public.issues(id) ON DELETE SET NULL;


--
-- Name: epics fk_9d480c64b2; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.epics
    ADD CONSTRAINT fk_9d480c64b2 FOREIGN KEY (start_date_sourcing_epic_id) REFERENCES public.epics(id) ON DELETE SET NULL;


--
-- Name: issues fk_a194299be1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issues
    ADD CONSTRAINT fk_a194299be1 FOREIGN KEY (moved_to_id) REFERENCES public.issues(id) ON DELETE SET NULL;


--
-- Name: subscription_add_on_purchases fk_a1db288990; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.subscription_add_on_purchases
    ADD CONSTRAINT fk_a1db288990 FOREIGN KEY (namespace_id) REFERENCES public.namespaces(id) ON DELETE CASCADE;


--
-- Name: merge_requests fk_a6963e8447; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merge_requests
    ADD CONSTRAINT fk_a6963e8447 FOREIGN KEY (target_project_id) REFERENCES public.projects(id) ON DELETE CASCADE;


--
-- Name: epics fk_aa5798e761; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.epics
    ADD CONSTRAINT fk_aa5798e761 FOREIGN KEY (closed_by_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: identities fk_aade90f0fc; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.identities
    ADD CONSTRAINT fk_aade90f0fc FOREIGN KEY (saml_provider_id) REFERENCES public.saml_providers(id) ON DELETE CASCADE;


--
-- Name: work_item_transitions fk_ac61084d25; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.work_item_transitions
    ADD CONSTRAINT fk_ac61084d25 FOREIGN KEY (moved_to_id) REFERENCES public.issues(id) ON DELETE SET NULL;


--
-- Name: merge_requests fk_ad525e1f87; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merge_requests
    ADD CONSTRAINT fk_ad525e1f87 FOREIGN KEY (merge_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: compliance_management_frameworks fk_b74c45b71f; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.compliance_management_frameworks
    ADD CONSTRAINT fk_b74c45b71f FOREIGN KEY (namespace_id) REFERENCES public.namespaces(id) ON DELETE CASCADE;


--
-- Name: issue_assignees fk_b7d881734a; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_assignees
    ADD CONSTRAINT fk_b7d881734a FOREIGN KEY (issue_id) REFERENCES public.issues(id) ON DELETE CASCADE;


--
-- Name: project_settings fk_bdc8715f08; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.project_settings
    ADD CONSTRAINT fk_bdc8715f08 FOREIGN KEY (duo_dependency_bump_breaking_changes_enabled_by_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: issues fk_c63cbf6c25; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issues
    ADD CONSTRAINT fk_c63cbf6c25 FOREIGN KEY (closed_by_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: issues fk_c78fbacd64; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issues
    ADD CONSTRAINT fk_c78fbacd64 FOREIGN KEY (namespace_id) REFERENCES public.namespaces(id) ON DELETE CASCADE;


--
-- Name: personal_access_tokens fk_c951fbf57e; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.personal_access_tokens
    ADD CONSTRAINT fk_c951fbf57e FOREIGN KEY (previous_personal_access_token_id) REFERENCES public.personal_access_tokens(id) ON DELETE SET NULL;


--
-- Name: subscription_add_on_purchases fk_caed789645; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.subscription_add_on_purchases
    ADD CONSTRAINT fk_caed789645 FOREIGN KEY (organization_id) REFERENCES public.organizations(id) ON DELETE CASCADE;


--
-- Name: todos fk_ccf0373936; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.todos
    ADD CONSTRAINT fk_ccf0373936 FOREIGN KEY (author_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: subscription_user_add_on_assignments fk_d1074a6e16; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.subscription_user_add_on_assignments
    ADD CONSTRAINT fk_d1074a6e16 FOREIGN KEY (organization_id) REFERENCES public.organizations(id) ON DELETE CASCADE;


--
-- Name: abuse_reports fk_d6848ca5d2; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.abuse_reports
    ADD CONSTRAINT fk_d6848ca5d2 FOREIGN KEY (reporter_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: users fk_d7b9ff90af; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT fk_d7b9ff90af FOREIGN KEY (organization_id) REFERENCES public.organizations(id);


--
-- Name: todos fk_d94154aa95; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.todos
    ADD CONSTRAINT fk_d94154aa95 FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: personal_access_tokens fk_da676c7ca5; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.personal_access_tokens
    ADD CONSTRAINT fk_da676c7ca5 FOREIGN KEY (organization_id) REFERENCES public.organizations(id) ON DELETE CASCADE;


--
-- Name: epics fk_dccd3f98fc; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.epics
    ADD CONSTRAINT fk_dccd3f98fc FOREIGN KEY (assignee_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: issues fk_df75a7c8b8; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issues
    ADD CONSTRAINT fk_df75a7c8b8 FOREIGN KEY (promoted_to_epic_id) REFERENCES public.epics(id) ON DELETE SET NULL;


--
-- Name: merge_requests fk_e719a85f8a; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merge_requests
    ADD CONSTRAINT fk_e719a85f8a FOREIGN KEY (author_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: award_emoji fk_e766b8f650; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.award_emoji
    ADD CONSTRAINT fk_e766b8f650 FOREIGN KEY (namespace_id) REFERENCES public.namespaces(id) ON DELETE CASCADE;


--
-- Name: events fk_eea90e3209; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.events
    ADD CONSTRAINT fk_eea90e3209 FOREIGN KEY (personal_namespace_id) REFERENCES public.namespaces(id) ON DELETE CASCADE;


--
-- Name: notes fk_eef74d5cc8; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notes
    ADD CONSTRAINT fk_eef74d5cc8 FOREIGN KEY (organization_id) REFERENCES public.organizations(id) ON DELETE CASCADE;


--
-- Name: emails fk_emails_user_id; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.emails
    ADD CONSTRAINT fk_emails_user_id FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: epics fk_epics_issue_id_with_on_delete_cascade; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.epics
    ADD CONSTRAINT fk_epics_issue_id_with_on_delete_cascade FOREIGN KEY (issue_id) REFERENCES public.issues(id) ON DELETE CASCADE;


--
-- Name: epics fk_epics_on_parent_id_with_on_delete_nullify; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.epics
    ADD CONSTRAINT fk_epics_on_parent_id_with_on_delete_nullify FOREIGN KEY (parent_id) REFERENCES public.epics(id) ON DELETE SET NULL;


--
-- Name: epics fk_f081aa4489; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.epics
    ADD CONSTRAINT fk_f081aa4489 FOREIGN KEY (group_id) REFERENCES public.namespaces(id) ON DELETE CASCADE;


--
-- Name: abuse_reports fk_f10de8b524; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.abuse_reports
    ADD CONSTRAINT fk_f10de8b524 FOREIGN KEY (resolved_by_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: abuse_reports fk_f748646298; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.abuse_reports
    ADD CONSTRAINT fk_f748646298 FOREIGN KEY (organization_id) REFERENCES public.organizations(id) ON DELETE CASCADE;


--
-- Name: work_item_transitions fk_f7c401aeb4; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.work_item_transitions
    ADD CONSTRAINT fk_f7c401aeb4 FOREIGN KEY (namespace_id) REFERENCES public.namespaces(id) ON DELETE CASCADE;


--
-- Name: work_item_positions fk_fb5f8027e0; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.work_item_positions
    ADD CONSTRAINT fk_fb5f8027e0 FOREIGN KEY (work_item_id) REFERENCES public.issues(id) ON DELETE CASCADE;


--
-- Name: member_roles fk_fc154c5d30; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.member_roles
    ADD CONSTRAINT fk_fc154c5d30 FOREIGN KEY (organization_id) REFERENCES public.organizations(id) ON DELETE CASCADE;


--
-- Name: issues fk_ffed080f01; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issues
    ADD CONSTRAINT fk_ffed080f01 FOREIGN KEY (updated_by_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: members fk_member_role_on_members; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.members
    ADD CONSTRAINT fk_member_role_on_members FOREIGN KEY (member_role_id) REFERENCES public.member_roles(id) ON DELETE SET NULL;


--
-- Name: personal_access_tokens fk_personal_access_tokens_user_id; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.personal_access_tokens
    ADD CONSTRAINT fk_personal_access_tokens_user_id FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: project_settings fk_project_settings_push_rule_id; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.project_settings
    ADD CONSTRAINT fk_project_settings_push_rule_id FOREIGN KEY (push_rule_id) REFERENCES public.push_rules(id) ON DELETE SET NULL;


--
-- Name: projects fk_projects_namespace_id; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.projects
    ADD CONSTRAINT fk_projects_namespace_id FOREIGN KEY (namespace_id) REFERENCES public.namespaces(id) ON DELETE RESTRICT;


--
-- Name: events fk_rails_0434b48643; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.events
    ADD CONSTRAINT fk_rails_0434b48643 FOREIGN KEY (project_id) REFERENCES public.projects(id) ON DELETE CASCADE;


--
-- Name: project_authorizations fk_rails_0f84bb11f3; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.project_authorizations
    ADD CONSTRAINT fk_rails_0f84bb11f3 FOREIGN KEY (project_id) REFERENCES public.projects(id) ON DELETE CASCADE;


--
-- Name: user_details fk_rails_12e0b3043d; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_details
    ADD CONSTRAINT fk_rails_12e0b3043d FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: catalog_resources fk_rails_16f09e5c44; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.catalog_resources
    ADD CONSTRAINT fk_rails_16f09e5c44 FOREIGN KEY (project_id) REFERENCES public.projects(id) ON DELETE CASCADE;


--
-- Name: work_item_parent_links fk_rails_231dba8959; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.work_item_parent_links
    ADD CONSTRAINT fk_rails_231dba8959 FOREIGN KEY (work_item_parent_id) REFERENCES public.issues(id) ON DELETE CASCADE;


--
-- Name: reviews fk_rails_29e6f859c4; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviews
    ADD CONSTRAINT fk_rails_29e6f859c4 FOREIGN KEY (author_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: saml_providers fk_rails_306d459be7; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.saml_providers
    ADD CONSTRAINT fk_rails_306d459be7 FOREIGN KEY (group_id) REFERENCES public.namespaces(id) ON DELETE CASCADE;


--
-- Name: namespace_settings fk_rails_3896d4fae5; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.namespace_settings
    ADD CONSTRAINT fk_rails_3896d4fae5 FOREIGN KEY (namespace_id) REFERENCES public.namespaces(id) ON DELETE CASCADE;


--
-- Name: reviews fk_rails_5ca11d8c31; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviews
    ADD CONSTRAINT fk_rails_5ca11d8c31 FOREIGN KEY (merge_request_id) REFERENCES public.merge_requests(id) ON DELETE CASCADE;


--
-- Name: work_item_parent_links fk_rails_601d5bec3a; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.work_item_parent_links
    ADD CONSTRAINT fk_rails_601d5bec3a FOREIGN KEY (work_item_id) REFERENCES public.issues(id) ON DELETE CASCADE;


--
-- Name: events fk_rails_61fbf6ca48; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.events
    ADD CONSTRAINT fk_rails_61fbf6ca48 FOREIGN KEY (group_id) REFERENCES public.namespaces(id) ON DELETE CASCADE;


--
-- Name: reviews fk_rails_64798be025; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviews
    ADD CONSTRAINT fk_rails_64798be025 FOREIGN KEY (project_id) REFERENCES public.projects(id) ON DELETE CASCADE;


--
-- Name: namespaces_sync_events fk_rails_9da32a0431; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.namespaces_sync_events
    ADD CONSTRAINT fk_rails_9da32a0431 FOREIGN KEY (namespace_id) REFERENCES public.namespaces(id) ON DELETE CASCADE;


--
-- Name: todos fk_rails_a27c483435; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.todos
    ADD CONSTRAINT fk_rails_a27c483435 FOREIGN KEY (group_id) REFERENCES public.namespaces(id) ON DELETE CASCADE;


--
-- Name: user_preferences fk_rails_a69bfcfd81; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_preferences
    ADD CONSTRAINT fk_rails_a69bfcfd81 FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: project_authorizations_for_migration fk_rails_b91fc9995e; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.project_authorizations_for_migration
    ADD CONSTRAINT fk_rails_b91fc9995e FOREIGN KEY (project_id) REFERENCES public.projects(id) ON DELETE CASCADE;


--
-- Name: projects_sync_events fk_rails_bbf0eef59f; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.projects_sync_events
    ADD CONSTRAINT fk_rails_bbf0eef59f FOREIGN KEY (project_id) REFERENCES public.projects(id) ON DELETE CASCADE;


--
-- Name: project_settings fk_rails_c6df6e6328; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.project_settings
    ADD CONSTRAINT fk_rails_c6df6e6328 FOREIGN KEY (project_id) REFERENCES public.projects(id) ON DELETE CASCADE;


--
-- Name: namespace_details fk_rails_cc11a451f8; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.namespace_details
    ADD CONSTRAINT fk_rails_cc11a451f8 FOREIGN KEY (namespace_id) REFERENCES public.namespaces(id) ON DELETE CASCADE;


--
-- Name: member_roles fk_rails_cf0ee35814; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.member_roles
    ADD CONSTRAINT fk_rails_cf0ee35814 FOREIGN KEY (namespace_id) REFERENCES public.namespaces(id) ON DELETE CASCADE;


--
-- Name: iterations_cadences fk_rails_ece400c55a; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.iterations_cadences
    ADD CONSTRAINT fk_rails_ece400c55a FOREIGN KEY (group_id) REFERENCES public.namespaces(id) ON DELETE CASCADE;


--
-- Name: merge_requests fk_source_project; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.merge_requests
    ADD CONSTRAINT fk_source_project FOREIGN KEY (source_project_id) REFERENCES public.projects(id) ON DELETE SET NULL;


--
-- PostgreSQL database dump complete
--


