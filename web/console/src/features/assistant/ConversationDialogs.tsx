import { Button, Modal } from '@/components/ui';
import type { AssistantConversation } from '@/domain/assistant';

interface ConversationDialogsProps {
  renameCandidate: AssistantConversation | null;
  renameTitle: string;
  renaming: boolean;
  deleteCandidate: AssistantConversation | null;
  deleting: boolean;
  onRenameTitleChange: (title: string) => void;
  onCloseRename: () => void;
  onRename: () => void;
  onCloseDelete: () => void;
  onDelete: () => void;
}

export function ConversationDialogs({
  renameCandidate,
  renameTitle,
  renaming,
  deleteCandidate,
  deleting,
  onRenameTitleChange,
  onCloseRename,
  onRename,
  onCloseDelete,
  onDelete,
}: ConversationDialogsProps) {
  return (
    <>
      <Modal title="重命名会话" isOpen={Boolean(renameCandidate)} onClose={onCloseRename}>
        <form
          className="space-y-5"
          onSubmit={(event) => {
            event.preventDefault();
            onRename();
          }}
        >
          <label className="field">
            <span>会话名称</span>
            <input
              autoFocus
              maxLength={160}
              value={renameTitle}
              onChange={(event) => onRenameTitleChange(event.target.value)}
            />
          </label>
          <div className="flex justify-end gap-2 border-t border-slate-200 pt-4">
            <Button type="button" variant="ghost" onClick={onCloseRename}>取消</Button>
            <Button
              type="submit"
              disabled={renaming || !renameTitle.trim() || renameTitle.trim() === renameCandidate?.title}
            >
              {renaming ? '保存中...' : '保存'}
            </Button>
          </div>
        </form>
      </Modal>

      <Modal title="删除会话" isOpen={Boolean(deleteCandidate)} onClose={onCloseDelete}>
        <div className="space-y-5">
          <p className="text-sm leading-6 text-slate-600">
            删除后，该会话中的消息将无法恢复。确定删除“
            <strong className="text-slate-900">{deleteCandidate?.title}</strong>
            ”吗？
          </p>
          <div className="flex justify-end gap-2 border-t border-slate-200 pt-4">
            <Button variant="ghost" onClick={onCloseDelete}>取消</Button>
            <Button variant="danger" disabled={deleting} onClick={onDelete}>
              {deleting ? '删除中...' : '确认删除'}
            </Button>
          </div>
        </div>
      </Modal>
    </>
  );
}
