import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  deleteComplianceRulesId,
  getGetComplianceRulesQueryKey,
  postComplianceRules,
  postCompliancePacksIdInstall,
  postComplianceRulesTest,
  putComplianceRulesId,
  useGetComplianceRules,
  useGetComplianceSchema,
} from '../../api/generated/compliance/compliance';
import { useGetFindings } from '../../api/generated/findings/findings';
import {
  ComplianceAttributeSpecType,
  ComplianceConditionOp,
  ComplianceRelatedClauseMode,
  Severity,
  type ComplianceAttributeSpec,
  type ComplianceCondition,
  type ComplianceRelatedClause,
  type ComplianceRule,
  type ComplianceRuleInput,
} from '../../api/model';
import { Button, IconButton } from '../../components/ui/Button';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { Dialog } from '../../components/ui/Dialog';
import { Panel } from '../../components/ui/Panel';
import { EmptyState, ErrorState, SkeletonRows } from '../../components/ui/states';
import { SeverityTag } from '../../components/ui/StatusDot';
import { EditIcon, PlusIcon, XIcon } from '../../components/icons';
import { toast } from '../../lib/toast';
import { Field, Select, SubHeader, TextInput, Toggle } from '../settings/parts';

type Draft = Omit<ComplianceRuleInput, 'conditions' | 'related'> & { conditions: ComplianceCondition[]; related: ComplianceRelatedClause[] };

const OPS: Record<ComplianceAttributeSpecType, ComplianceConditionOp[]> = {
  string: [ComplianceConditionOp.eq, ComplianceConditionOp.neq, ComplianceConditionOp.contains, ComplianceConditionOp.not_contains, ComplianceConditionOp.regex, ComplianceConditionOp.exists, ComplianceConditionOp.days_left_lt, ComplianceConditionOp.days_left_gt],
  string_array: [ComplianceConditionOp.eq, ComplianceConditionOp.neq, ComplianceConditionOp.contains, ComplianceConditionOp.not_contains, ComplianceConditionOp.exists],
  number: [ComplianceConditionOp.eq, ComplianceConditionOp.neq, ComplianceConditionOp.gt, ComplianceConditionOp.lt, ComplianceConditionOp.exists],
  boolean: [ComplianceConditionOp.eq, ComplianceConditionOp.neq, ComplianceConditionOp.exists],
};

const emptyDraft: Draft = {
  name: '', connectorType: '', entityKind: '', conditions: [], related: [], severity: Severity.warning,
  title: '', remediationLink: '', enabled: false,
};

function isDaysLeftOp(op: ComplianceConditionOp) {
  return op === ComplianceConditionOp.days_left_lt || op === ComplianceConditionOp.days_left_gt;
}

function valueFor(type: ComplianceAttributeSpecType, value: unknown) {
  if (type === ComplianceAttributeSpecType.boolean) return String(value === true);
  if (type === ComplianceAttributeSpecType.string_array) return Array.isArray(value) ? value.join(', ') : '';
  return value == null ? '' : String(value);
}

function parseValue(type: ComplianceAttributeSpecType, value: string): unknown {
  if (type === ComplianceAttributeSpecType.boolean) return value === 'true';
  if (type === ComplianceAttributeSpecType.number) return Number(value);
  if (type === ComplianceAttributeSpecType.string_array) return value.split(',').map((item) => item.trim()).filter(Boolean);
  return value;
}

function ConditionRow({ conditions, onChange, onRemove, attributeList }: { conditions: ComplianceCondition[]; onChange: (index: number, next: Partial<ComplianceCondition>) => void; onRemove: (index: number) => void; attributeList: ComplianceAttributeSpec[] }) {
  const { t } = useTranslation();
  return (
    <>
      {conditions.map((condition, index) => {
        const attribute = attributeList.find((item) => item.name === condition.attribute);
        const type = attribute?.type ?? ComplianceAttributeSpecType.string;
        const daysLeft = condition.op === ComplianceConditionOp.days_left_lt || condition.op === ComplianceConditionOp.days_left_gt;
        const updateValue = (value: string) => onChange(index, { value: daysLeft ? (value === '' ? '' : Number(value)) : parseValue(type, value) });
        return <div key={index} className="mb-2 grid gap-2 sm:grid-cols-[1fr_0.7fr_1fr_auto]"><Select aria-label={t('compliance.attribute')} value={condition.attribute} onChange={(e) => onChange(index, { attribute: e.target.value, op: OPS[attributeList.find((item) => item.name === e.target.value)?.type ?? ComplianceAttributeSpecType.string][0], value: '' })}><option value="">{t('compliance.attribute')}</option>{attributeList.map((item) => <option key={item.name} value={item.name}>{item.name}</option>)}</Select><Select aria-label={t('compliance.operator')} value={condition.op} onChange={(e) => { const op = e.target.value as ComplianceConditionOp; onChange(index, isDaysLeftOp(op) === daysLeft ? { op } : { op, value: '' }); }}>{OPS[type].map((op) => <option key={op} value={op}>{op === ComplianceConditionOp.days_left_lt ? t('compliance.daysLeftLt') : op === ComplianceConditionOp.days_left_gt ? t('compliance.daysLeftGt') : op}</option>)}</Select>{condition.op === ComplianceConditionOp.exists ? <span /> : type === ComplianceAttributeSpecType.boolean ? <Select aria-label={t('compliance.value')} value={valueFor(type, condition.value)} onChange={(e) => onChange(index, { value: parseValue(type, e.target.value) })}><option value="true">true</option><option value="false">false</option></Select> : <TextInput aria-label={t('compliance.value')} type={daysLeft || type === ComplianceAttributeSpecType.number ? 'number' : 'text'} step={daysLeft ? 1 : undefined} placeholder={type === ComplianceAttributeSpecType.string_array ? t('compliance.arrayHint') : undefined} value={valueFor(type, condition.value)} onChange={(e) => updateValue(e.target.value)} />}<IconButton label={t('compliance.removeCondition')} onClick={() => onRemove(index)}><XIcon size={15} /></IconButton></div>;
      })}
    </>
  );
}

export function RulesPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const rules = useGetComplianceRules();
  const findings = useGetFindings({ page: 1, pageSize: 500, status: 'open', checkType: 'compliance' });
  const schema = useGetComplianceSchema();
  const [editing, setEditing] = useState<ComplianceRule | 'new' | null>(null);
  const [draft, setDraft] = useState<Draft>(emptyDraft);
  const [error, setError] = useState('');
  const [preview, setPreview] = useState<{ connectorName: string; entities: Record<string, unknown>[] }[] | null>(null);
  const [toDelete, setToDelete] = useState<ComplianceRule | null>(null);

  const kinds = useMemo(() => Object.keys(schema.data?.attributes?.[draft.connectorType] ?? {}), [schema.data, draft.connectorType]);
  const attributes = schema.data?.attributes?.[draft.connectorType]?.[draft.entityKind] ?? [];
  const joinFields = schema.data?.joinFields ?? [];
  const violations = useMemo(() => {
    const counts = new Map<string, number>();
    for (const finding of findings.data?.items ?? []) if (finding.ruleId) counts.set(finding.ruleId, (counts.get(finding.ruleId) ?? 0) + 1);
    return counts;
  }, [findings.data]);
  const invalidate = () => queryClient.invalidateQueries({ queryKey: getGetComplianceRulesQueryKey() });
  const close = () => { setEditing(null); setError(''); setPreview(null); };
  const payload = (): ComplianceRuleInput => ({ ...draft, remediationLink: draft.remediationLink?.trim() || undefined });
  const clausesValid = draft.related.every((clause) => clause.mode && clause.connectorType && clause.entityKind && clause.join.sourceField && clause.join.relatedField && (!clause.conditions || clause.conditions.every((c) => c.attribute && c.op)));
  const valid = draft.name.trim() && draft.connectorType && draft.entityKind && draft.title.trim() && draft.conditions.length > 0 && draft.conditions.every((condition) => condition.attribute && condition.op) && clausesValid;

  const save = useMutation({
    mutationFn: () => editing === 'new' ? postComplianceRules(payload()) : putComplianceRulesId(editing!.id, payload()),
    onSuccess: () => { invalidate(); close(); toast.success(t('compliance.saved')); },
    onError: () => setError(t('compliance.saveError')),
  });
  const remove = useMutation({
    mutationFn: (id: string) => deleteComplianceRulesId(id),
    onSuccess: () => { invalidate(); setToDelete(null); toast.success(t('compliance.deleted')); },
    onError: () => toast.error(t('compliance.deleteError')),
  });
  const installPack = useMutation({
    mutationFn: () => postCompliancePacksIdInstall('recommended'),
    onSuccess: () => { invalidate(); toast.success(t('compliance.packInstalled')); },
    onError: () => toast.error(t('compliance.packInstallError')),
  });
  const testRule = useMutation({
    mutationFn: () => postComplianceRulesTest(payload()),
    onSuccess: (result) => setPreview(result.items),
    onError: () => setError(t('compliance.testError')),
  });

  const edit = (rule: ComplianceRule | 'new') => {
    setEditing(rule);
    setError(''); setPreview(null);
    setDraft(rule === 'new' ? emptyDraft : { ...rule, remediationLink: rule.remediationLink ?? '', related: rule.related ?? [] });
  };
  const setType = (connectorType: string) => setDraft((current) => ({ ...current, connectorType, entityKind: '', conditions: [], related: [] }));
  const setKind = (entityKind: string) => setDraft((current) => ({ ...current, entityKind, conditions: [], related: [] }));
  const updateCondition = (index: number, next: Partial<ComplianceCondition>) => setDraft((current) => ({ ...current, conditions: current.conditions.map((condition, i) => i === index ? { ...condition, ...next, ...(next.op === ComplianceConditionOp.exists ? { value: true } : {}) } : condition) }));
  const addCondition = () => setDraft((current) => ({ ...current, conditions: [...current.conditions, { attribute: '', op: ComplianceConditionOp.eq, value: '' }] }));
  const addClause = () => setDraft((current) => ({ ...current, related: [...current.related, { mode: ComplianceRelatedClauseMode.requires, connectorType: '', entityKind: '', join: { sourceField: '', relatedField: '' }, conditions: [] }] }));
  const removeClause = (index: number) => setDraft((current) => ({ ...current, related: current.related.filter((_, i) => i !== index) }));
  const updateClause = (index: number, next: Partial<ComplianceRelatedClause>) => setDraft((current) => {
    const clause = current.related[index];
    const updated = { ...clause, ...next };
    if (next.connectorType !== undefined && next.connectorType !== clause.connectorType) updated.entityKind = '';
    if (next.connectorType !== undefined || next.entityKind !== undefined) updated.join = { ...updated.join, relatedField: '' };
    if (next.connectorType !== undefined || next.entityKind !== undefined) updated.conditions = [];
    return { ...current, related: current.related.map((c, i) => i === index ? updated : c) };
  });
  const updateClauseCondition = (clauseIndex: number, condIndex: number, next: Partial<ComplianceCondition>) => setDraft((current) => ({ ...current, related: current.related.map((clause, ci) => ci === clauseIndex ? { ...clause, conditions: (clause.conditions ?? []).map((condition, i) => i === condIndex ? { ...condition, ...next, ...(next.op === ComplianceConditionOp.exists ? { value: true } : {}) } : condition) } : clause) }));
  const addClauseCondition = (clauseIndex: number) => setDraft((current) => ({ ...current, related: current.related.map((clause, ci) => ci === clauseIndex ? { ...clause, conditions: [...(clause.conditions ?? []), { attribute: '', op: ComplianceConditionOp.eq, value: '' }] } : clause) }));

  return <div>
    <SubHeader title={t('compliance.title')} description={t('compliance.subtitle')} />
    <div className="mb-4 flex justify-end"><Button variant="primary" size="sm" onClick={() => edit('new')}><PlusIcon size={14} />{t('compliance.new')}</Button></div>
    <Panel>
      {rules.isLoading ? <SkeletonRows rows={4} /> : rules.isError || !rules.data ? <ErrorState description={t('compliance.loadError')} onRetry={() => rules.refetch()} /> : rules.data.items.length === 0 ? <EmptyState title={t('compliance.emptyTitle')} description={t('compliance.emptyDescription')} action={<Button variant="secondary" size="sm" disabled={installPack.isPending} onClick={() => installPack.mutate()}>{t('compliance.installRecommended')}</Button>} /> :
        <ul className="divide-y divide-line-soft">{rules.data.items.map((rule) => <li key={rule.id} className="flex items-center gap-3 px-4 py-3">
          <Toggle checked={Boolean(rule.enabled)} onChange={(enabled) => { putComplianceRulesId(rule.id, { ...rule, enabled }).then(invalidate).catch(() => toast.error(t('compliance.saveError'))); }} label={t('compliance.enabledLabel', { name: rule.name })} size="sm" />
          <div className="min-w-0 flex-1"><p className="truncate text-sm font-medium text-ink">{rule.name}</p><p className="font-mono text-2xs text-ink-faint">{rule.connectorType} · {rule.entityKind}</p></div>
          <span className="font-mono text-2xs text-ink-faint">{t('compliance.violations', { count: violations.get(rule.id) ?? 0 })}</span><SeverityTag severity={rule.severity} /><IconButton label={t('compliance.edit')} onClick={() => edit(rule)}><EditIcon size={15} /></IconButton><IconButton label={t('compliance.delete')} onClick={() => setToDelete(rule)} className="hover:text-err"><XIcon size={15} /></IconButton>
        </li>)}</ul>}
    </Panel>
    <Dialog open={editing !== null} onClose={close} title={editing === 'new' ? t('compliance.createTitle') : t('compliance.editTitle')} size="lg">
      <form className="space-y-4" onSubmit={(event) => { event.preventDefault(); if (!valid) { setError(t('compliance.validation')); return; } save.mutate(); }}>
        <Field label={t('compliance.name')} htmlFor="rule-name"><TextInput id="rule-name" value={draft.name} onChange={(e) => setDraft((d) => ({ ...d, name: e.target.value }))} /></Field>
        <div className="grid gap-4 sm:grid-cols-2"><Field label={t('compliance.connectorType')} htmlFor="rule-type"><Select id="rule-type" value={draft.connectorType} onChange={(e) => setType(e.target.value)}><option value="">{t('compliance.selectType')}</option>{Object.keys(schema.data?.attributes ?? {}).map((type) => <option key={type}>{type}</option>)}</Select></Field>
        <Field label={t('compliance.entityKind')} htmlFor="rule-kind"><Select id="rule-kind" disabled={!draft.connectorType} value={draft.entityKind} onChange={(e) => setKind(e.target.value)}><option value="">{t('compliance.selectKind')}</option>{kinds.map((kind) => <option key={kind}>{kind}</option>)}</Select></Field></div>
        <div><p className="mb-1.5 font-mono text-2xs text-ink-faint">{t('compliance.conditions')}</p><ConditionRow conditions={draft.conditions} onChange={updateCondition} onRemove={(i) => setDraft((current) => ({ ...current, conditions: current.conditions.filter((_, idx) => idx !== i) }))} attributeList={attributes} /><Button type="button" variant="secondary" size="sm" disabled={!draft.entityKind} onClick={addCondition}><PlusIcon size={14} />{t('compliance.addCondition')}</Button></div>
        <div><p className="mb-1.5 font-mono text-2xs text-ink-faint">{t('compliance.relatedTitle')}</p><p className="mb-3 text-2xs text-ink-faint">{t('compliance.relatedHint')}</p>{draft.related.map((clause, clauseIndex) => {
          const sourceAttributes = schema.data?.attributes?.[draft.connectorType]?.[draft.entityKind] ?? [];
          const relatedAttributes = schema.data?.attributes?.[clause.connectorType]?.[clause.entityKind] ?? [];
          const relatedKindsForClause = Object.keys(schema.data?.attributes?.[clause.connectorType] ?? {});
          const sourceFieldOptions = [...joinFields, ...sourceAttributes.map((a) => `attributes.${a.name}`)];
          const relatedFieldOptions = [...joinFields, ...relatedAttributes.map((a) => `attributes.${a.name}`)];
          return <div key={clauseIndex} className="relative mb-3 rounded-md border border-line-soft p-3"><div className="mb-2 grid gap-2 sm:grid-cols-2"><Select aria-label={t('compliance.clauseMode')} value={clause.mode} onChange={(e) => updateClause(clauseIndex, { mode: e.target.value as ComplianceRelatedClauseMode })}><option value={ComplianceRelatedClauseMode.requires}>{t('compliance.clauseModeRequires')}</option><option value={ComplianceRelatedClauseMode.forbids}>{t('compliance.clauseModeForbids')}</option></Select><Select aria-label={t('compliance.clauseConnectorType')} value={clause.connectorType} onChange={(e) => updateClause(clauseIndex, { connectorType: e.target.value })}><option value="">{t('compliance.clauseSelectField')}</option>{Object.keys(schema.data?.attributes ?? {}).map((type) => <option key={type}>{type}</option>)}</Select></div><div className="mb-2 grid gap-2 sm:grid-cols-2"><Select aria-label={t('compliance.clauseEntityKind')} disabled={!clause.connectorType} value={clause.entityKind} onChange={(e) => updateClause(clauseIndex, { entityKind: e.target.value })}><option value="">{t('compliance.clauseSelectField')}</option>{relatedKindsForClause.map((kind) => <option key={kind}>{kind}</option>)}</Select><div className="grid gap-2 sm:grid-cols-2"><Select aria-label={t('compliance.clauseJoinSourceField')} value={clause.join.sourceField} onChange={(e) => updateClause(clauseIndex, { join: { ...clause.join, sourceField: e.target.value } })}><option value="">{t('compliance.clauseSelectField')}</option>{sourceFieldOptions.map((f) => <option key={f}>{f}</option>)}</Select><Select aria-label={t('compliance.clauseJoinRelatedField')} value={clause.join.relatedField} onChange={(e) => updateClause(clauseIndex, { join: { ...clause.join, relatedField: e.target.value } })}><option value="">{t('compliance.clauseSelectField')}</option>{relatedFieldOptions.map((f) => <option key={f}>{f}</option>)}</Select></div></div>{clause.conditions && clause.conditions.length > 0 && <div className="mb-2"><p className="mb-1.5 text-2xs font-medium text-ink-faint">{t('compliance.conditions')}</p><ConditionRow conditions={clause.conditions} onChange={(i, next) => updateClauseCondition(clauseIndex, i, next)} onRemove={(i) => setDraft((current) => ({ ...current, related: current.related.map((c, ci) => ci === clauseIndex ? { ...c, conditions: (c.conditions ?? []).filter((_, idx) => idx !== i) } : c) }))} attributeList={relatedAttributes} /></div>}<Button type="button" variant="secondary" size="sm" disabled={!clause.entityKind} onClick={() => addClauseCondition(clauseIndex)}><PlusIcon size={14} />{t('compliance.addCondition')}</Button><IconButton label={t('compliance.removeClause')} onClick={() => removeClause(clauseIndex)} className="absolute right-3 top-3"><XIcon size={15} /></IconButton></div>;
        })}<Button type="button" variant="secondary" size="sm" disabled={!draft.entityKind || draft.related.length >= 5} onClick={addClause} title={draft.related.length >= 5 ? t('compliance.clauseMaxReached') : ''}><PlusIcon size={14} />{t('compliance.addClause')}</Button></div>
        <div className="grid gap-4 sm:grid-cols-2"><Field label={t('compliance.severity')} htmlFor="rule-severity"><Select id="rule-severity" value={draft.severity} onChange={(e) => setDraft((d) => ({ ...d, severity: e.target.value as Severity }))}>{Object.values(Severity).map((severity) => <option key={severity}>{severity}</option>)}</Select></Field><Field label={t('compliance.titleLabel')} htmlFor="rule-title"><TextInput id="rule-title" value={draft.title} onChange={(e) => setDraft((d) => ({ ...d, title: e.target.value }))} /></Field></div>
        <Field label={t('compliance.remediation')} htmlFor="rule-remediation"><TextInput id="rule-remediation" type="url" value={draft.remediationLink ?? ''} onChange={(e) => setDraft((d) => ({ ...d, remediationLink: e.target.value }))} /></Field>
        <Toggle checked={Boolean(draft.enabled)} onChange={(enabled) => setDraft((d) => ({ ...d, enabled }))} label={t('compliance.enabled')} />
        {error && <p role="alert" className="text-sm text-err">{error}</p>}
        {preview && <div className="rounded-md border border-line-soft p-3 text-sm"><p className="font-medium">{t('compliance.previewTitle')}</p>{preview.length ? <ul className="mt-1 list-disc pl-5">{preview.map((match) => <li key={match.connectorName}>{match.connectorName}: {match.entities.length}</li>)}</ul> : <p className="mt-1 text-ink-muted">{t('compliance.noMatches')}</p>}</div>}
        <div className="flex justify-end gap-2"><Button type="button" variant="secondary" disabled={!valid || testRule.isPending} onClick={() => { if (!valid) { setError(t('compliance.validation')); return; } testRule.mutate(); }}>{t('compliance.test')}</Button><Button type="submit" variant="primary" disabled={save.isPending}>{t('common.save')}</Button></div>
      </form>
    </Dialog>
    <ConfirmDialog open={toDelete !== null} onClose={() => setToDelete(null)} onConfirm={() => toDelete && remove.mutate(toDelete.id)} title={t('compliance.deleteTitle')} description={t('compliance.deleteDescription', { name: toDelete?.name })} confirmLabel={t('common.delete')} cancelLabel={t('common.cancel')} tone="danger" confirmDisabled={remove.isPending} />
  </div>;
}
