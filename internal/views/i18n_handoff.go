package views

var handoffTranslations = map[string][3]string{
	"Choose Paste Terminal": {"选择粘贴目标终端", "選擇貼上目標終端機", "貼り付け先の端末を選択"},
	"Paste %d to %s":        {"粘贴 %d 条到 %s", "貼上 %d 則至 %s", "%d 件を %s に貼り付け"},
	"Paste comments without pressing Enter. Drag to a terminal to choose another target.": {"仅粘贴评论，不发送回车。拖到指定终端可更换目标。", "僅貼上評論，不傳送 Enter。拖曳至指定終端機可更換目標。", "Enter を送らずコメントを貼り付けます。別の端末にドラッグして対象を選べます。"},
	"Comments belong to another project. Nothing was pasted.":                             {"评论属于其他项目，未粘贴任何内容。", "評論屬於其他專案，未貼上任何內容。", "別のプロジェクトのコメントです。何も貼り付けていません。"},
	"The selected comments changed. Select them again.":                                   {"所选评论已变化，请重新选择。", "所選評論已變更，請重新選取。", "選択したコメントが変わりました。選択し直してください。"},
	"Comments kept in the terminal draft; nothing was sent.":                              {"评论已保留在终端草稿中，尚未发送。", "評論已保留於終端機草稿中，尚未傳送。", "コメントを端末の下書きに保存しました。まだ送信していません。"},
	"Comments pasted. Press Enter in the terminal when ready.":                            {"评论已粘贴，准备好后在终端按回车。", "評論已貼上，準備好後在終端機按 Enter。", "コメントを貼り付けました。準備ができたら端末で Enter を押してください。"},
	"Comments pending · %s":  {"待粘贴评论 · %s", "待貼上評論 · %s", "貼り付け待ちのコメント · %s"},
	"Retry Paste":            {"重试粘贴", "重試貼上", "貼り付けを再試行"},
	"Dismiss Terminal Draft": {"关闭终端评论草稿", "關閉終端機評論草稿", "端末のコメント下書きを閉じる"},
	"Safe multiline paste is unavailable here. Nothing was sent; comments stay in this draft.": {"此终端暂不支持安全的多行粘贴。尚未发送，评论保留在此草稿中。", "此終端機暫不支援安全的多行貼上。尚未傳送，評論保留於此草稿。", "この端末では安全な複数行貼り付けが使えません。未送信のコメントをここに保持しています。"},
	"Terminal Comment Draft %s":                                            {"终端评论草稿 %s", "終端機評論草稿 %s", "端末のコメント下書き %s"},
	"Drag selected comments to a terminal":                                 {"将所选评论拖到终端", "將所選評論拖曳至終端機", "選択したコメントを端末にドラッグ"},
	"Drag this comment to a terminal":                                      {"将此评论拖到终端", "將此評論拖曳至終端機", "このコメントを端末にドラッグ"},
	"Terminal Drop Target %s":                                              {"终端拖放目标 %s", "終端機拖放目標 %s", "端末のドロップ先 %s"},
	"Drop to paste into %s · Enter is not sent":                            {"松开后粘贴到 %s · 不发送回车", "放開後貼上至 %s · 不傳送 Enter", "%s にドロップして貼り付け · Enter は送信しません"},
	"Enter to rename · Esc to cancel · Leave blank for the automatic name": {"回车重命名 · Esc 取消 · 留空使用自动名称", "Enter 重新命名 · Esc 取消 · 留空使用自動名稱", "Enter で変更 · Esc で取消 · 空欄は自動名"},
	"Enter for a new line · ⌘Enter to save":                                {"回车换行 · ⌘Enter 保存", "Enter 換行 · ⌘Enter 儲存", "Enter で改行 · ⌘Enter で保存"},
	"Select diff lines, or choose Entire File, to save this comment.":      {"请选择差异行或“整个文件”，以保存此评论。", "請選取差異行或「整個檔案」，以儲存此評論。", "コメントを保存するには差分の行または「ファイル全体」を選んでください。"},
	"Comment Save Hint":                                                    {"评论保存提示", "評論儲存提示", "コメント保存のヒント"},
}
