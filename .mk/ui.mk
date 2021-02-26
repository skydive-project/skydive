UI_DIRS := \
	statics/ui/index.html \
	statics/ui/css/* \
	statics/ui/css/themes/*/* \
	statics/ui/fonts/* \
	statics/ui/img/* \
	statics/ui/js/*

UI_V2_DIRS := \
	statics/ui_v2/index.html \
	statics/ui_v2/assets \
	statics/ui_v2/dist \
	statics/ui_v2/fonts

.PHONY: .ui
.ui: .ui_v2

.PHONY: .ui_v2
.ui_v2:
	cd statics/ui_v2 && npm install && npm run prepare && cd -
