(ns bills-to-beans.core
    (:require [reagent.core :as r :refer [atom]]
              [reagent.session :as session]
              [bills-to-beans.transaction :refer [<new-transaction-page>]]
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
  [<new-transaction-page>])

(defn <notes> []
  [:div.container
   [:div.row
    [:div.col-sm-3.pull-right
     [:p "Usually:"]
     [:table.table
      [:tbody
       [:tr [:td "- Assets"] [:td "→"] [:td "+ Expenses"]]
       [:tr [:td "- Income"] [:td "→"] [:td "+ Assets"]]
       ]]
     ]]])

;; -------------------------
;; Initialize app

(defn mount-root []
  (r/render [<flash>] (.getElementById js/document "flash"))
  (r/render [<home-page>] (.getElementById js/document "app"))
  (r/render [<notes>] (.getElementById js/document "notes"))
  )

(defn init! []
  (mount-root))
