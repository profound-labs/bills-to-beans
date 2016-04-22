(ns bills-to-beans.core
    (:require [reagent.core :as r :refer [atom]]
              [reagent.session :as session]
              [bills-to-beans.bill :refer [<bill-upload-page>]]
              [secretary.core :as secretary :include-macros true]))

;; -------------------------
;; Views

(defn home-page []
  [:div [<bill-upload-page>]])

;; -------------------------
;; Initialize app

(defn mount-root []
  (r/render [home-page] (.getElementById js/document "app")))

(defn init! []
  (mount-root))
