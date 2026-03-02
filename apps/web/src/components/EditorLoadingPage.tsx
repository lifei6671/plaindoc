import { LoaderCircle } from "lucide-react";

interface EditorLoadingPageProps {
  description?: string;
}

export function EditorLoadingPage({
  description = "正在加载文档中..."
}: EditorLoadingPageProps) {
  return (
    <div className="admin-auth-page" role="status" aria-live="polite" aria-busy="true">
      <div className="flex items-center gap-2 text-sm text-slate-600">
        <LoaderCircle size={16} className="animate-spin" />
        <span>{description}</span>
      </div>
    </div>
  );
}
