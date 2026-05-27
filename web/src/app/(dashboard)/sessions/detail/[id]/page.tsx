import SessionDetailClient from "@/components/session-detail/session-detail-client";

export function generateStaticParams() {
  return [{ id: "0" }];
}

type PageProps = {
  params: Promise<{ id: string }>;
};

export default async function SessionDetailPage({ params }: PageProps) {
  const { id } = await params;
  const sessionId = Number(id);
  return <SessionDetailClient sessionId={sessionId} />;
}