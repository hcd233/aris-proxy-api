"use client";

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type Dispatch,
  type SetStateAction,
} from "react";

interface UseOptimisticUpdateOptions<T> {
  /** 列表 state setter；乐观更新与失败回滚都会通过它改写列表 */
  setItems: Dispatch<SetStateAction<T[]>>;
  /** 行定位 key：返回该行的唯一标识（如 id / name），用于定位回滚与禁用控件 */
  getKey: (item: T) => string | number;
  /** 实际写入后端；参数为乐观应用 patch 后的新对象，失败时需抛出错误 */
  update: (item: T) => Promise<void>;
  /** 成功后回调（如 toast.success） */
  onSuccess?: (item: T) => void;
  /** 失败后回调（如 showErrorToast）；行回滚由 hook 自动完成 */
  onError?: (err: unknown, original: T) => void;
}

/**
 * 列表页行级状态切换的统一乐观更新 hook。
 *
 * 点击 Switch / 徽章等行内控件时立即在本地应用 patch（UI 即时响应，避免
 * "重新拉整表 + 骨架屏替换"造成的闪烁），再异步写入后端；失败时自动回滚
 * 该行并触发 onError。
 *
 * 用法：
 *   const toggle = useOptimisticUpdate<ModelItem>({
 *     setItems: setModels,
 *     getKey: (m) => m.id,
 *     update: async (m) => { await api.updateModel(m.id, { enabled: m.enabled }); },
 *     onSuccess: (m) => toast.success(m.enabled ? t("models.enabled") : t("models.disabled")),
 *     onError: (err) => showErrorToast(err, { title: t("models.toggle_error") }),
 *   });
 *   <Switch
 *     checked={model.enabled}
 *     disabled={toggle.updatingKey !== null}
 *     onCheckedChange={() => toggle.apply(model, { enabled: !model.enabled })}
 *   />
 *
 * 单飞语义：一次只允许一个在途请求，updatingKey 标记当前正在更新的行；
 * 期间新的 apply 调用被忽略（防快速双击竞态）。建议控件 disabled 用
 * `updatingKey !== null` 全局禁用，避免“点击被静默忽略”的困惑。
 */
export function useOptimisticUpdate<T>(options: UseOptimisticUpdateOptions<T>) {
  const [updatingKey, setUpdatingKey] = useState<string | number | null>(null);
  const optionsRef = useRef(options);
  const busyRef = useRef(false);

  useEffect(() => {
    optionsRef.current = options;
  });

  const apply = useCallback((item: T, patch: Partial<T>) => {
    if (busyRef.current) return;
    const { setItems, getKey, update, onSuccess, onError } = optionsRef.current;
    const key = getKey(item);
    const optimistic = { ...item, ...patch };

    busyRef.current = true;
    setUpdatingKey(key);
    // 乐观应用：立即本地更新该行
    setItems((list) => list.map((i) => (getKey(i) === key ? optimistic : i)));

    update(optimistic)
      .then(() => onSuccess?.(optimistic))
      .catch((err) => {
        // 失败回滚：恢复原对象
        setItems((list) => list.map((i) => (getKey(i) === key ? item : i)));
        onError?.(err, item);
      })
      .finally(() => {
        busyRef.current = false;
        setUpdatingKey(null);
      });
  }, []);

  return { apply, updatingKey };
}
