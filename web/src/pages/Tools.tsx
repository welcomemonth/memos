import { WrenchIcon } from "lucide-react";

const Tools = () => {
  return (
    <section className="mx-auto w-full max-w-5xl min-h-full flex flex-col justify-start items-start sm:pt-3 md:pt-6 pb-8">
      <div className="w-full px-4 sm:px-6">
        <div className="w-full rounded-xl border border-border bg-background px-6 py-6">
          <div className="flex items-center gap-3 mb-4">
            <WrenchIcon className="w-6 h-6 text-primary" />
            <h1 className="text-2xl font-semibold tracking-tight text-foreground">工具集</h1>
          </div>
          <p className="text-muted-foreground mb-6">这里将放置各种实用小工具。</p>

          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <div className="rounded-lg border border-border bg-muted/40 px-4 py-6 text-center hover:bg-muted/60 transition-colors cursor-pointer">
              <WrenchIcon className="w-8 h-8 mx-auto mb-3 text-muted-foreground" />
              <h3 className="font-medium text-foreground">待添加</h3>
              <p className="text-sm text-muted-foreground mt-1">更多工具即将上线</p>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
};

export default Tools;
