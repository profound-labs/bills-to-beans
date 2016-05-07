(ns bills-to-beans.core
    (:require [reagent.core :as r :refer [atom]]
              [reagent.session :as session]
              [bills-to-beans.bills :refer [<new-bill-page>]]
              [secretary.core :as secretary :include-macros true]
              [clojure.string :as string]))

;; -------------------------
;; Views

(defn <flash-message> []
  (let [class (if-let [class (:class (session/get :flash))] class "alert-info" )
        message (:msg (session/get :flash))]

    (when (not (string/blank? message))
      [:div.alert.alert-dismissable
       {:class class :role "alert"}
       [:button.close {:type "button" :aria-label "close" :on-click #(session/put! :flash nil)}
        [:span {:aria-hidden "true"} "×"]];; &times;
       (when (not (string/blank? message))
         [:span message])
       ])
    ))

(defn <flash> []
  [:div.container
   [:div.row.col-md-6.col-md-offset-3
    [<flash-message>]]])

(defn <home-page> []
  [<new-bill-page>])

;; -------------------------
;; Initialize app

(defn mount-root []
  (r/render [<flash>] (.getElementById js/document "flash"))
  (r/render [<home-page>] (.getElementById js/document "app")))

(defn init! []
  (mount-root))
