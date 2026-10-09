import type { TFunction } from 'i18next';

const actionKeys: Record<string, string> = {
  'connector.action': 'connectorAction',
  'connector.recipe_actions_changed': 'recipeActionsChanged',
  'backup.import': 'backupImport',
  'runbook.run.step_resent': 'stepResent',
  'runbook.run.step_marked_done': 'stepMarkedDone',
};

export function auditActionLabel(action: string, t: TFunction): string {
  const key = actionKeys[action];
  return key ? t(`journal.actionLabels.${key}`) : action;
}
