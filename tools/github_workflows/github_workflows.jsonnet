local workflows_template = import 'tools/github_workflows/workflows_template.libsonnet';

workflows_template.getWorkflows(
  ['bb_remote_asset'],
  ['bb_remote_asset:bb_remote_asset'],
)
