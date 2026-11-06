// Swagger UI has no tag groups: read x-tagGroups from the spec and put a title
// that opens and closes the group above its first tag.
function TagGroups() {
  return {
    wrapComponents: {
      OperationTag: (Original, system) => (props) => {
        const { createElement: h, Fragment } = system.React;
        const groups = system.specSelectors.specJson().get("x-tagGroups");
        const group = groups && groups.find((g) => g.get("tags").includes(props.tag));
        if (!group) return h(Original, props);

        const name = group.get("name");
        const key = ["tag-groups", name];
        const open = system.layoutSelectors.isShown(key, false);
        const toggle = () => system.layoutActions.show(key, !open);
        const count = group.get("tags").size;
        const title = group.getIn(["tags", 0]) === props.tag && h(
          "h2",
          { className: "tag-group" },
          h(
            "button",
            { type: "button", "aria-expanded": open, onClick: toggle },
            name,
            h("span", null, count === 1 ? "1 tag" : `${count} tags`),
          ),
        );

        // A filter shows its matches even in a closed group: only the first tag has the title.
        const filter = system.layoutSelectors.currentFilter(); // true until something is typed
        const visible = open || (typeof filter === "string" && filter !== "");
        return h(Fragment, null, title, visible && h("div", { className: "tag-group-body" }, h(Original, props)));
      },
    },
  };
}
