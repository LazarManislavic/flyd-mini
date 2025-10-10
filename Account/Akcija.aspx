<%@ Page Title="" Language="C#" MasterPageFile="~/Site.Master" AutoEventWireup="true" CodeBehind="Akcija.aspx.cs" Inherits="WebApplication6.Account.Akcija" %>
<asp:Content ID="Content1" ContentPlaceHolderID="MainContent" runat="server">

    <asp:GridView ID="GridView1" runat="server" AutoGenerateColumns="False" DataSourceID="SqlDataSource1">
        <Columns>
            <asp:BoundField DataField="Sifra_proizvoda" HeaderText="Sifra_proizvoda" SortExpression="Sifra_proizvoda"></asp:BoundField>
            <asp:BoundField DataField="Akciska_cena" HeaderText="Akciska_cena" SortExpression="Akciska_cena"></asp:BoundField>
        </Columns>
    </asp:GridView>
    <asp:SqlDataSource runat="server" ID="SqlDataSource1" ConnectionString="<%$ ConnectionStrings:lazarConnectionString %>" SelectCommand="SELECT [Sifra_proizvoda], [Akciska_cena] FROM [akcije]"></asp:SqlDataSource><br />
    <asp:Label ID="Label1" runat="server" Text="Za akciske cene molimo kontaktirajte nas korisnicki servis na broj:+38269-43-69-76"></asp:Label>

</asp:Content>
