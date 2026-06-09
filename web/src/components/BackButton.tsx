import { useNavigate } from "react-router-dom";
import { ArrowLeftIcon } from "lucide-react";
import { Routes } from "@/router";

// 返回 tools 页面（默认，不需要传参）
/* <BackButton />

// 返回其他页面
<BackButton to="/" />           // 回到首页
<BackButton to="/files" />      // 回到文件页面
<BackButton to={Routes.HOME} />  // 使用路由常量 */


interface BackButtonProps {
  /** 点击后导航到的目标路径，默认 /tools */
  to?: string;
  /** aria-label，默认 "返回上一页" */
  label?: string;
}

const BackButton = ({ to = Routes.TOOLS, label = "返回上一页" }: BackButtonProps) => {
  const navigate = useNavigate();

  return (
    <button
      onClick={() => navigate(to)}
      className="inline-flex items-center justify-center rounded-md hover:bg-muted transition-colors p-1 shrink-0"
      aria-label={label}
    >
      <ArrowLeftIcon className="w-5 h-auto text-muted-foreground" />
    </button>
  );
};

export default BackButton;
