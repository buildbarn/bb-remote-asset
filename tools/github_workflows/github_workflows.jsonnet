local workflows_template = import 'external/com_github_buildbarn_bb_storage/tools/github_workflows/workflows_template.libsonnet';

workflows_template.getWorkflows(
  ['bb_remote_asset'],
  ['bb_remote_asset:bb_remote_asset'],
)
