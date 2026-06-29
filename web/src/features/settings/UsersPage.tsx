/**
 * Settings → Users (operator-only). User roster with create/invite, role change,
 * enable/disable, password reset, and delete. The route is already role-gated;
 * this page also guards defensively and the server enforces the real boundary.
 */
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  useGetUsers,
  getGetUsersQueryKey,
  postUsers,
  patchUsersUserId,
  deleteUsersUserId,
  postUsersUserIdResetPassword,
} from '../../api/generated/users/users';
import { useGetMe } from '../../api/generated/me/me';
import type { Role, User } from '../../api/model';
import { Button } from '../../components/ui/Button';
import { Panel } from '../../components/ui/Panel';
import { SkeletonRows, ErrorState, EmptyState } from '../../components/ui/states';
import { Dialog } from '../../components/ui/Dialog';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { ToneTag } from '../../components/ui/ToneTag';
import { toast } from '../../lib/toast';
import { SubHeader, Field, TextInput, Select } from './parts';
import { PlusIcon, LayersIcon } from '../../components/icons';

export function UsersPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { data: me } = useGetMe();
  const { data, isLoading, isError, refetch } = useGetUsers();
  const [inviteOpen, setInviteOpen] = useState(false);
  const [toDelete, setToDelete] = useState<User | null>(null);

  const invalidate = () => queryClient.invalidateQueries({ queryKey: getGetUsersQueryKey() });

  const patch = useMutation({
    mutationFn: ({ id, role, disabled }: { id: string; role?: Role; disabled?: boolean }) =>
      patchUsersUserId(id, { role, disabled }),
    onSuccess: () => {
      invalidate();
      toast.success(t('settings.users.updated'));
    },
    onError: () => toast.error(t('settings.users.updateError')),
  });

  const remove = useMutation({
    mutationFn: (id: string) => deleteUsersUserId(id),
    onSuccess: () => {
      invalidate();
      toast.success(t('settings.users.deleted'));
      setToDelete(null);
    },
    onError: () => toast.error(t('settings.users.deleteError')),
  });

  const reset = useMutation({
    mutationFn: (id: string) => postUsersUserIdResetPassword(id),
    onSuccess: () => toast.success(t('settings.users.resetSent')),
    onError: () => toast.error(t('settings.users.resetError')),
  });

  return (
    <div>
      <SubHeader title={t('settings.users.title')} description={t('settings.users.subtitle')} />

      <div className="mb-4 flex justify-end">
        <Button variant="primary" size="sm" onClick={() => setInviteOpen(true)}>
          <PlusIcon size={15} />
          {t('settings.users.invite')}
        </Button>
      </div>

      <Panel>
        {isLoading ? (
          <SkeletonRows rows={5} />
        ) : isError || !data ? (
          <ErrorState description={t('settings.users.loadError')} onRetry={() => refetch()} />
        ) : data.length === 0 ? (
          <EmptyState icon={<LayersIcon size={18} />} title={t('settings.users.emptyTitle')} />
        ) : (
          <ul className="divide-y divide-line-soft">
            {data.map((u) => {
              const isSelf = u.id === me?.id;
              return (
                <li key={u.id} className="flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3">
                  <div className="min-w-0 flex-1">
                    <p className="flex items-center gap-2 text-sm text-ink">
                      <span className="truncate font-medium">{u.displayName || u.username}</span>
                      <span className="font-mono text-2xs text-ink-faint">@{u.username}</span>
                      {u.disabled && <ToneTag tone="idle" label={t('common.disabled')} />}
                    </p>
                    <p className="mt-0.5 flex items-center gap-1.5 font-mono text-2xs text-ink-faint">
                      <span>{u.email ?? '—'}</span>
                      <span>·</span>
                      <span>{u.authSource}</span>
                    </p>
                  </div>

                  <Select
                    aria-label={t('settings.users.role')}
                    value={u.role}
                    disabled={isSelf || patch.isPending}
                    onChange={(e) => patch.mutate({ id: u.id, role: e.target.value as Role })}
                    className="w-32"
                  >
                    <option value="viewer">{t('settings.users.roleViewer')}</option>
                    <option value="operator">{t('settings.users.roleOperator')}</option>
                  </Select>

                  <div className="flex items-center gap-1.5">
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={patch.isPending || isSelf}
                      onClick={() => patch.mutate({ id: u.id, disabled: !u.disabled })}
                    >
                      {u.disabled ? t('common.enable') : t('common.disable')}
                    </Button>
                    {u.authSource === 'local' && (
                      <Button
                        variant="ghost"
                        size="sm"
                        disabled={reset.isPending}
                        onClick={() => reset.mutate(u.id)}
                      >
                        {t('settings.users.reset')}
                      </Button>
                    )}
                    <Button
                      variant="danger"
                      size="sm"
                      disabled={isSelf || remove.isPending}
                      onClick={() => setToDelete(u)}
                    >
                      {t('common.delete')}
                    </Button>
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </Panel>

      <InviteDialog open={inviteOpen} onClose={() => setInviteOpen(false)} onCreated={invalidate} />

      <ConfirmDialog
        open={toDelete !== null}
        onClose={() => setToDelete(null)}
        onConfirm={() => toDelete && remove.mutate(toDelete.id)}
        tone="danger"
        title={t('settings.users.deleteTitle')}
        description={t('settings.users.deleteConfirm')}
        confirmLabel={t('common.delete')}
        cancelLabel={t('common.cancel')}
        confirmDisabled={remove.isPending}
      />
    </div>
  );
}

function InviteDialog({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: () => void;
}) {
  const { t } = useTranslation();
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [role, setRole] = useState<Role>('viewer');

  const create = useMutation({
    mutationFn: () => postUsers({ username, email: email || undefined, role }),
    onSuccess: () => {
      onCreated();
      toast.success(t('settings.users.created'));
      setUsername('');
      setEmail('');
      setRole('viewer');
      onClose();
    },
    onError: () => toast.error(t('settings.users.createError')),
  });

  return (
    <Dialog open={open} onClose={onClose} title={t('settings.users.inviteTitle')}>
      <form
        className="space-y-4"
        onSubmit={(e) => {
          e.preventDefault();
          if (username.trim()) create.mutate();
        }}
      >
        <Field label={t('settings.users.username')} htmlFor="inv-username">
          <TextInput
            id="inv-username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            autoFocus
          />
        </Field>
        <Field label={t('settings.users.email')} htmlFor="inv-email">
          <TextInput
            id="inv-email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </Field>
        <Field label={t('settings.users.role')} htmlFor="inv-role">
          <Select id="inv-role" value={role} onChange={(e) => setRole(e.target.value as Role)}>
            <option value="viewer">{t('settings.users.roleViewer')}</option>
            <option value="operator">{t('settings.users.roleOperator')}</option>
          </Select>
        </Field>
        <div className="flex justify-end gap-2 pt-1">
          <Button type="button" variant="ghost" size="sm" onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button
            type="submit"
            variant="primary"
            size="sm"
            disabled={!username.trim() || create.isPending}
          >
            {t('settings.users.sendInvite')}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
