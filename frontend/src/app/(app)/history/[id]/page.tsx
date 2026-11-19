import { SessionDetail } from "@/components/history/session-detail";

export default async function HistoryDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <SessionDetail id={id} />;
}
